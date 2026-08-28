package powerdns

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	freecache "github.com/coocood/freecache"
	"github.com/stretchr/testify/assert"
)

// A single client is shared across the whole provider while Terraform walks the
// resource graph in parallel, so version detection must happen exactly once no
// matter how many resources issue their first request simultaneously.
func TestAPIVersionDetectedOnceUnderConcurrency(t *testing.T) {
	var detections int32

	client := &PowerDNSClient{
		serverID: "localhost",
		BaseClient: &BaseClient{
			ServerURL:  "https://pdns.example.test",
			APIKey:     "test-key",
			APIVersion: -1,
			HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/v1/servers" {
					atomic.AddInt32(&detections, 1)
				}
				return jsonResponse(http.StatusOK, `[]`), nil
			})},
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.ListZones(context.Background())
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&detections),
		"API version should be probed exactly once across concurrent callers")
}

// A version already present on the client is authoritative and must not trigger
// a probe.
func TestAPIVersionPresetSkipsDetection(t *testing.T) {
	var detections int32

	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/servers" {
			atomic.AddInt32(&detections, 1)
		}
		return jsonResponse(http.StatusOK, `[]`), nil
	})

	_, err := client.ListZones(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	assert.Zero(t, atomic.LoadInt32(&detections))
}

func newCachingTestClient(fn roundTripFunc) *PowerDNSClient {
	return &PowerDNSClient{
		serverID: "localhost",
		BaseClient: &BaseClient{
			ServerURL:   "https://pdns.example.test",
			APIKey:      "test-key",
			APIVersion:  1,
			HTTP:        &http.Client{Transport: fn},
			CacheEnable: true,
			CacheSize:   1024 * 1024,
			Cache:       freecache.NewCache(1024 * 1024),
			CacheTTL:    30,
		},
	}
}

const oneRecordZone = `{"id":"example.com.","name":"example.com.","rrsets":[` +
	`{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1"}]}]}`

// Writing a record must drop the zone from the cache; otherwise the read that
// Terraform issues straight after the write is served the pre-write zone and
// recorded as drift.
func TestWriteInvalidatesZoneCache(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, int32(1), atomic.LoadInt32(&listCalls)) {
		return
	}

	// Second read is served from cache.
	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, int32(1), atomic.LoadInt32(&listCalls), "second read should hit the cache") {
		return
	}

	_, err = client.ReplaceRecordSet(ctx, "example.com.", ResourceRecordSet{
		Name:    "www.example.com.",
		Type:    "A",
		TTL:     300,
		Records: []Record{{Content: "192.0.2.2"}},
	})
	if !assert.NoError(t, err) {
		return
	}

	// The write invalidated the zone, so this must go back to the server.
	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(2), atomic.LoadInt32(&listCalls),
		"read after write should refetch instead of serving the stale zone")
}

func TestDeleteRecordSetInvalidatesZoneCache(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NoError(t, client.DeleteRecordSet(ctx, "example.com.", "www.example.com.", "A")) {
		return
	}

	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(2), atomic.LoadInt32(&listCalls))
}

// DNS names are case-insensitive, so two config sites naming one zone with
// different capitalisation must share a cache entry rather than each getting
// their own.
func TestZoneCacheKeyIsCaseInsensitive(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	_, err = client.ListRecords(ctx, "Example.COM.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&listCalls),
		"a case variant of a cached zone should hit the same cache entry")
}

// A write must invalidate the zone whatever capitalisation it is spelled with,
// or the stale entry under the other spelling is served as drift.
func TestWriteInvalidatesZoneCacheAcrossCase(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	_, err = client.ReplaceRecordSet(ctx, "EXAMPLE.com.", ResourceRecordSet{
		Name:    "www.example.com.",
		Type:    "A",
		TTL:     300,
		Records: []Record{{Content: "192.0.2.2"}},
	})
	if !assert.NoError(t, err) {
		return
	}

	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(2), atomic.LoadInt32(&listCalls),
		"a write spelled with different case should still invalidate the zone")
}

// Zone variants are distinct zones in PowerDNS: the name is case-insensitive
// but the variant suffix is an API identifier and must stay verbatim.
func TestZoneCacheKeyPreservesVariant(t *testing.T) {
	assert.Equal(t, "example.com.", string(zoneCacheKey("Example.COM.")))

	// The base is lowered, the variant suffix is left exactly as given.
	assert.Equal(t, "example.com..internal", string(zoneCacheKey("Example.COM..internal")))

	// A variant is a different zone from the plain name and must not collide.
	assert.NotEqual(t, string(zoneCacheKey("example.com.")), string(zoneCacheKey("example.com..internal")))
}

// Path segments reach the API as user input and must not be able to alter the
// request path.
func TestEndpointsEscapePathSegments(t *testing.T) {
	var gotPath string

	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.EscapedPath()
		return jsonResponse(http.StatusOK, `{"kind":"X","metadata":[]}`), nil
	})

	_, err := client.GetZoneMetadata(context.Background(), "ex ample.com.", "ALLOW-AXFR-FROM")
	if !assert.NoError(t, err) {
		return
	}
	assert.NotContains(t, gotPath, " ")
	assert.Contains(t, gotPath, "ex%20ample.com.")
}

// A zone that is absent has no records; that is not an error, but any other
// failure status must be reported rather than read as an empty zone.
func TestListRecordsMissingZoneIsEmptyButErrorsSurface(t *testing.T) {
	missing := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})
	records, err := missing.ListRecords(context.Background(), "gone.example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Empty(t, records)

	unauthorized := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, `{"error":"Unauthorized"}`), nil
	})
	_, err = unauthorized.ListRecords(context.Background(), "example.com.")
	if !assert.Error(t, err) {
		return
	}
	assert.True(t, strings.Contains(err.Error(), "Unauthorized"))
}
