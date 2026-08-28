package powerdns

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/coocood/freecache"
	"github.com/stretchr/testify/assert"
)

const oneRecordZone = `{"name":"example.com.","kind":"Native","rrsets":[{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1","disabled":false}]}]}`

// newCachingTestClient returns a client with the request cache switched on, so
// a second lookup of the same zone is served from cache instead of the server.
func newCachingTestClient(fn roundTripFunc) *PowerDNSClient {
	client := newTestClient(fn)
	client.CacheEnable = true
	client.Cache = freecache.NewCache(1024 * 1024)
	client.CacheTTL = 60
	return client
}

// DNS names are case-insensitive, so two config sites naming one zone with
// different capitalisation must share a cache entry rather than each getting
// their own. Seeding the cache under one spelling and reading it back under
// another asserts that directly, without depending on ListRecords' cache-miss
// path (which on this code treats an ordinary miss as a hard error).
func TestZoneCacheKeyIsCaseInsensitive(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&listCalls, 1)
		return jsonResponse(http.StatusOK, oneRecordZone), nil
	})

	ctx := context.Background()

	if !assert.NoError(t, client.Cache.Set(zoneCacheKey("example.com."), []byte(oneRecordZone), client.CacheTTL)) {
		return
	}

	zoneInfo, err := client.GetZoneInfoFromCache(ctx, "Example.COM.")
	if !assert.NoError(t, err, "a case variant should find the cached entry") {
		return
	}
	if assert.NotNil(t, zoneInfo) {
		assert.Equal(t, "example.com.", zoneInfo.Name)
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(&listCalls),
		"the cached entry should be served without contacting the server")
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
