package powerdns

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A cold cache is the normal first read, not a failure.
func TestListRecordsColdCacheFallsThroughToAPI(t *testing.T) {
	calls := 0
	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, `{"name":"example.com.","rrsets":[
			{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1","disabled":false}]}
		]}`), nil
	})

	records, err := client.ListRecords(context.Background(), "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Len(t, records, 1)
	assert.Equal(t, 1, calls)
}

// The second read of the same zone is served from cache.
func TestListRecordsSecondReadUsesCache(t *testing.T) {
	calls := 0
	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, `{"name":"example.com.","rrsets":[
			{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1","disabled":false}]}
		]}`), nil
	})

	_, err := client.ListRecords(context.Background(), "example.com.")
	assert.NoError(t, err)
	_, err = client.ListRecords(context.Background(), "example.com.")
	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "second read should come from cache")
}

// A write must drop the cached copy, otherwise the next read in the same run
// is served a pre-write snapshot.
func TestWriteInvalidatesZoneCache(t *testing.T) {
	calls := 0
	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch {
			return jsonResponse(http.StatusNoContent, ``), nil
		}
		calls++
		return jsonResponse(http.StatusOK, `{"name":"example.com.","rrsets":[
			{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1","disabled":false}]}
		]}`), nil
	})

	_, err := client.ListRecords(context.Background(), "example.com.")
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)

	_, err = client.ReplaceRecordSet(context.Background(), "example.com.", ResourceRecordSet{
		Name: "new.example.com.", Type: "A", TTL: 300, ChangeType: "REPLACE",
	})
	assert.NoError(t, err)

	_, err = client.ListRecords(context.Background(), "example.com.")
	assert.NoError(t, err)
	assert.Equal(t, 2, calls, "read after a write must hit the API again")
}

func TestDeleteZoneInvalidatesCache(t *testing.T) {
	calls := 0
	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodDelete {
			return jsonResponse(http.StatusNoContent, ``), nil
		}
		calls++
		return jsonResponse(http.StatusOK, `{"name":"example.com.","rrsets":[]}`), nil
	})

	_, err := client.ListRecords(context.Background(), "example.com.")
	assert.NoError(t, err)
	assert.NoError(t, client.DeleteZone(context.Background(), "example.com."))

	_, err = client.ListRecords(context.Background(), "example.com.")
	assert.NoError(t, err)
	assert.Equal(t, 2, calls, "read after a delete must hit the API again")
}
