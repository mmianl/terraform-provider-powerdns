package powerdns

import (
	"context"
	"crypto/tls"
	"os"
	"testing"
)

// newCacheAccClient builds a live client against the acceptance-test PowerDNS
// (PDNS_SERVER_URL / PDNS_API_KEY), with caching toggled per the argument.
func newCacheAccClient(t *testing.T, cache bool) *PowerDNSClient {
	t.Helper()
	c, err := NewPowerDNSClient(
		context.Background(),
		os.Getenv("PDNS_SERVER_URL"),
		os.Getenv("PDNS_API_KEY"),
		&tls.Config{},
		cache, "0", 300,
	)
	if err != nil {
		t.Fatalf("NewPowerDNSClient: %v", err)
	}
	return c
}

// TestAccZoneCache_HitAndInvalidate verifies, against a live PowerDNS, that the
// zone memo serves a cached (stale) read after an out-of-band change, and that
// invalidateZoneCache forces a refetch. A second, cache-disabled client makes
// the out-of-band change so the cached client's memo is not auto-invalidated.
func TestAccZoneCache_HitAndInvalidate(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test; set TF_ACC=1 (plus PDNS_SERVER_URL / PDNS_API_KEY)")
	}
	testAccPreCheck(t)

	ctx := context.Background()
	const (
		zone   = "cache-acc-test.example."
		recID  = "host.cache-acc-test.example.:::A"
		recFQN = "host.cache-acc-test.example."
	)

	cached := newCacheAccClient(t, true)
	direct := newCacheAccClient(t, false)

	// Clean any leftover from a prior failed run, then create the zone fresh.
	_ = direct.DeleteZone(ctx, zone)
	if _, err := direct.CreateZone(ctx, ZoneInfo{
		Name:        zone,
		Kind:        "Native",
		Nameservers: []string{"ns1.example."},
	}); err != nil {
		t.Fatalf("CreateZone: %v", err)
	}
	t.Cleanup(func() { _ = direct.DeleteZone(ctx, zone) })

	writeA := func(content string) {
		t.Helper()
		if _, err := direct.ReplaceRecordSet(ctx, zone, ResourceRecordSet{
			Name:       recFQN,
			Type:       "A",
			ChangeType: "REPLACE",
			TTL:        300,
			Records:    []Record{{Content: content}},
		}); err != nil {
			t.Fatalf("ReplaceRecordSet(%s): %v", content, err)
		}
	}

	readA := func() string {
		t.Helper()
		rr, err := cached.GetRecordSetByID(ctx, zone, recID)
		if err != nil {
			t.Fatalf("GetRecordSetByID: %v", err)
		}
		if rr == nil || len(rr.Records) == 0 {
			t.Fatalf("record %s not found", recID)
		}
		return rr.Records[0].Content
	}

	writeA("10.0.0.1")

	// First read populates the cached client's memo.
	if got := readA(); got != "10.0.0.1" {
		t.Fatalf("initial read: got %q, want 10.0.0.1", got)
	}

	// Change the record out-of-band via the cache-disabled client. The cached
	// client's memo is untouched, so it must still serve the old value.
	writeA("10.0.0.2")
	if got := readA(); got != "10.0.0.1" {
		t.Fatalf("after out-of-band change, cached read should be stale 10.0.0.1, got %q", got)
	}

	// Invalidating the cached client's memo must force a refetch.
	cached.invalidateZoneCache(zone)
	if got := readA(); got != "10.0.0.2" {
		t.Fatalf("after invalidate, read should be fresh 10.0.0.2, got %q", got)
	}
}
