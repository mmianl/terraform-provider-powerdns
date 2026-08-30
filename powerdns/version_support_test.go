package powerdns

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
)

// Feature availability differs across PowerDNS releases and backends, so the
// tests that depend on one ask the server rather than assuming. This keeps a
// run against an older server meaningful instead of failing on things that
// release genuinely cannot do.
var (
	serverCapsOnce sync.Once
	serverCaps     struct {
		views    bool
		comments bool
		catalog  bool
	}
)

// detectServerCapabilities probes the running authoritative server once.
func detectServerCapabilities(t *testing.T) {
	t.Helper()

	serverCapsOnce.Do(func() {
		serverURL := os.Getenv("PDNS_SERVER_URL")
		apiKey := os.Getenv("PDNS_API_KEY")
		if serverURL == "" || apiKey == "" {
			return
		}

		client, err := NewPowerDNSClient(context.Background(), serverURL, os.Getenv("PDNS_SERVER_ID"), apiKey, nil, false, "0", 0)
		if err != nil {
			return
		}

		// Views and networks were added in PowerDNS 5.0, and even where the
		// endpoint exists the server rejects writes unless it runs the LMDB
		// backend with views enabled and a zone cache. Listing therefore is not
		// enough to tell whether these resources can actually be managed, so
		// this probes with a real write and undoes it.
		serverCaps.views = probeViewSupport(client)

		// Record comments need a backend that stores them. LMDB only gained
		// that in 5.1, and the API reports it as a 422 on the write itself, so
		// this probes with a real zone.
		serverCaps.comments = probeCommentSupport(client)
		serverCaps.catalog = probeCatalogSupport(client)
	})
}

// probeViewSupport tries an actual network write, which is what the resources
// do. A server that lists views but cannot accept one is no use to them.
func probeViewSupport(client *PowerDNSClient) bool {
	ctx := context.Background()

	req, err := client.newRequest(ctx, http.MethodGet, client.serverEndpoint("/views"), nil)
	if err != nil {
		return false
	}
	resp, err := client.HTTP.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	const probeNetwork = "198.51.100.0"
	const probePrefix = "24"
	if err := client.SetNetwork(ctx, probeNetwork, probePrefix, "capability-probe"); err != nil {
		return false
	}
	_ = client.DeleteNetwork(ctx, probeNetwork, probePrefix)

	return true
}

// probeCommentSupport creates a throwaway zone and tries to attach a comment.
func probeCommentSupport(client *PowerDNSClient) bool {
	ctx := context.Background()
	zone := "capability-probe.invalid."

	// Ignore failures here: a leftover zone from an interrupted run is fine,
	// and a create failure means the probe simply reports no support.
	_ = client.DeleteZone(ctx, zone)
	if _, err := client.CreateZone(ctx, ZoneInfo{Name: zone, Kind: "Native", Nameservers: []string{}}); err != nil {
		return false
	}
	defer func() { _ = client.DeleteZone(ctx, zone) }()

	comments := []Comment{{Content: "capability probe"}}
	_, err := client.ReplaceRecordSet(ctx, zone, ResourceRecordSet{
		Name:       "probe." + zone,
		Type:       "A",
		TTL:        300,
		ChangeType: "REPLACE",
		Records:    []Record{{Name: "probe." + zone, Type: "A", Content: "192.0.2.1", TTL: 300}},
		Comments:   &comments,
	})

	return err == nil
}

// probeCatalogSupport checks whether the server keeps a catalog assignment.
// PowerDNS before 4.7 accepts the field on create and then drops it, so asking
// for it back is the only reliable signal.
func probeCatalogSupport(client *PowerDNSClient) bool {
	ctx := context.Background()
	zone := "catalog-probe.invalid."

	_ = client.DeleteZone(ctx, zone)
	created, err := client.CreateZone(ctx, ZoneInfo{
		Name:        zone,
		Kind:        "Master",
		Catalog:     "catalog-probe-parent.invalid.",
		Nameservers: []string{},
	})
	if err != nil {
		return false
	}
	defer func() { _ = client.DeleteZone(ctx, zone) }()

	return created.Catalog != ""
}

// testAccPreCheckCatalog skips a test when the server drops catalog zones.
func testAccPreCheckCatalog(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)
	detectServerCapabilities(t)
	if !serverCaps.catalog {
		t.Skip("server does not support catalog zones (PowerDNS 4.7+)")
	}
}

// testAccPreCheckViews skips a test when the server has no views support.
func testAccPreCheckViews(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)
	detectServerCapabilities(t)
	if !serverCaps.views {
		t.Skip("server does not support views and networks (PowerDNS 5.0+ with the LMDB backend)")
	}
}

// testAccPreCheckComments skips a test when the backend cannot store comments.
func testAccPreCheckComments(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)
	detectServerCapabilities(t)
	if !serverCaps.comments {
		t.Skip("backend does not support record comments (LMDB gained these in PowerDNS 5.1)")
	}
}
