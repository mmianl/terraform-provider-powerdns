package powerdns

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// holdFirstZoneRead lets the first zone GET reach the server and read its
// answer, then parks it before the caller sees the response. That is the
// moment a slow read is carrying a snapshot taken before a concurrent write.
type holdFirstZoneRead struct {
	inner   http.RoundTripper
	armed   int32
	started chan struct{}
	release chan struct{}
}

func newHoldFirstZoneRead(inner http.RoundTripper) *holdFirstZoneRead {
	return &holdFirstZoneRead{
		inner:   inner,
		armed:   1,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *holdFirstZoneRead) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := h.inner.RoundTrip(r)
	if err != nil || r.Method != http.MethodGet || !strings.Contains(r.URL.RawQuery, "rrsets=true") {
		return resp, err
	}
	if !atomic.CompareAndSwapInt32(&h.armed, 1, 0) {
		return resp, err
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	close(h.started)
	<-h.release
	return resp, nil
}

// staleReadScenario runs a read that is parked mid-flight, completes a write
// while it is parked, then lets the read finish. The read's answer predates the
// write, so it must not be what the next read is served from the cache.
func staleReadScenario(t *testing.T, client *PowerDNSClient, hold *holdFirstZoneRead, zone string, write func() error) {
	t.Helper()
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1)
	var readErr error
	go func() {
		defer wg.Done()
		_, readErr = client.ListRecords(ctx, zone)
	}()

	select {
	case <-hold.started:
	case <-time.After(10 * time.Second):
		t.Fatal("the first read never reached the server")
	}

	writeDone := make(chan error, 1)
	go func() { writeDone <- write() }()

	// The write must be able to finish while the read is still parked: a
	// slow read holding up writers would turn one slow request into a stall.
	var writeErr error
	select {
	case writeErr = <-writeDone:
	case <-time.After(3 * time.Second):
		t.Log("write did not finish while a read was in flight; releasing the read")
		close(hold.release)
		writeErr = <-writeDone
		wg.Wait()
		assert.NoError(t, writeErr)
		assert.NoError(t, readErr)
		t.Fatal("write was blocked behind an in-flight read")
	}
	assert.NoError(t, writeErr)

	close(hold.release)
	wg.Wait()
	assert.NoError(t, readErr)

	records, err := client.ListRecords(ctx, zone)
	assert.NoError(t, err)

	found := false
	for _, rec := range records {
		if rec.Name == "new."+zone {
			found = true
		}
	}
	assert.True(t, found, "a read that overlapped a write left its pre-write snapshot in the cache")
}

func TestListRecordsOverlappingWriteDoesNotCacheStaleSnapshot(t *testing.T) {
	var written int32
	hold := newHoldFirstZoneRead(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch {
			atomic.StoreInt32(&written, 1)
			return jsonResponse(http.StatusNoContent, ``), nil
		}
		rrsets := `{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1","disabled":false}]}`
		if atomic.LoadInt32(&written) == 1 {
			rrsets += `,{"name":"new.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.2","disabled":false}]}`
		}
		return jsonResponse(http.StatusOK, `{"name":"example.com.","rrsets":[`+rrsets+`]}`), nil
	}))

	client := newCachingTestClient(hold.RoundTrip)

	staleReadScenario(t, client, hold, "example.com.", func() error {
		_, err := client.ReplaceRecordSet(context.Background(), "example.com.", ResourceRecordSet{
			Name: "new.example.com.", Type: "A", TTL: 300,
			Records: []Record{{Content: "192.0.2.2"}},
		})
		return err
	})
}

// The same overlap against a real PowerDNS, so the answer the parked read is
// holding is a genuine pre-write zone and the write is a genuine PATCH.
func TestListRecordsOverlappingWriteAgainstServer(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set; skipping acceptance test")
	}
	testAccPreCheck(t)

	ctx := context.Background()
	client, err := NewPowerDNSClient(ctx, os.Getenv("PDNS_SERVER_URL"), os.Getenv("PDNS_SERVER_ID"), os.Getenv("PDNS_API_KEY"), nil, true, "10", 300, 60)
	if !assert.NoError(t, err) {
		return
	}

	zone := fmt.Sprintf("race%d.example.", time.Now().UnixNano())
	_, err = client.CreateZone(ctx, ZoneInfo{Name: zone, Kind: "Native", Nameservers: []string{"ns1." + zone}})
	if !assert.NoError(t, err) {
		return
	}
	t.Cleanup(func() { _ = client.DeleteZone(ctx, zone) })

	hold := newHoldFirstZoneRead(client.HTTP.Transport)
	client.HTTP.Transport = hold

	staleReadScenario(t, client, hold, zone, func() error {
		_, err := client.ReplaceRecordSet(ctx, zone, ResourceRecordSet{
			Name: "new." + zone, Type: "A", TTL: 300,
			Records: []Record{{Content: "192.0.2.2"}},
		})
		return err
	})
}
