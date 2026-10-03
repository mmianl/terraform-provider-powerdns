package powerdns

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Readers hammer one zone while a writer rewrites a record each round, with
// nothing held back: the overlap happens or not on its own. A read that starts
// after a write has returned must see that write, and once everything has
// settled the cache must agree with the server. Errors from the server itself
// are counted apart, since they say nothing about the cache.
func TestCacheReadsAndWritesRaceAgainstServer(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set; skipping acceptance test")
	}
	testAccPreCheck(t)

	ctx := context.Background()
	client, err := NewPowerDNSClient(ctx, os.Getenv("PDNS_SERVER_URL"), os.Getenv("PDNS_SERVER_ID"), os.Getenv("PDNS_API_KEY"), nil, true, "100", 300, 60)
	if err != nil {
		t.Fatal(err)
	}

	zone := fmt.Sprintf("stress%d.example.", time.Now().UnixNano())
	if _, err := client.CreateZone(ctx, ZoneInfo{Name: zone, Kind: "Native", Nameservers: []string{"ns1." + zone}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.DeleteZone(ctx, zone) })

	name := "x." + zone
	has := func(recs []Record, content string) bool {
		for _, r := range recs {
			if r.Name == name && r.Content == content {
				return true
			}
		}
		return false
	}

	const rounds = 400
	const readers = 16
	var stale, serverErrs int32

	for i := 0; i < rounds; i++ {
		content := fmt.Sprintf("10.%d.%d.1", i/256, i%256)
		// Start cold, so reads go to the server while the write is under way.
		client.invalidateZoneCache(zone)
		var written int32
		stop := make(chan struct{})
		var wg sync.WaitGroup

		for r := 0; r < readers; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					after := atomic.LoadInt32(&written) == 1
					recs, err := client.ListRecords(ctx, zone)
					if err != nil {
						atomic.AddInt32(&serverErrs, 1)
						continue
					}
					if after && !has(recs, content) {
						atomic.AddInt32(&stale, 1)
					}
				}
			}()
		}

		if _, err := client.ReplaceRecordSet(ctx, zone, ResourceRecordSet{
			Name: name, Type: "A", TTL: 300, Records: []Record{{Content: content}},
		}); err != nil {
			atomic.AddInt32(&serverErrs, 1)
			close(stop)
			wg.Wait()
			continue
		}
		atomic.StoreInt32(&written, 1)
		time.Sleep(time.Millisecond)
		close(stop)
		wg.Wait()

		recs, err := client.ListRecords(ctx, zone)
		if err != nil {
			atomic.AddInt32(&serverErrs, 1)
		} else if !has(recs, content) {
			atomic.AddInt32(&stale, 1)
		}
	}

	t.Logf("server errors (ignored): %d", atomic.LoadInt32(&serverErrs))
	if n := atomic.LoadInt32(&stale); n > 0 {
		t.Errorf("%d reads that began after a completed write did not see it", n)
	}
}
