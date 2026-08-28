package powerdns

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newRecursorTestClient(fn roundTripFunc) *RecursorClient {
	return &RecursorClient{BaseClient: &BaseClient{
		ServerURL:  "https://pdns.example.test",
		APIKey:     "test-key",
		APIVersion: 1,
		HTTP:       &http.Client{Transport: fn},
	}}
}

// The recursor reports an unknown forward zone with a 422 and an explanatory
// message rather than a 404, so absence is recognised by matching that message
// inside the error statusError builds. This test pins the two together: if the
// wording on either side drifts apart, a missing zone starts surfacing as a
// hard failure and Terraform reports an error instead of planning a create.
func TestGetForwardZoneTreats422AsNotFound(t *testing.T) {
	client := newRecursorTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		return jsonResponse(http.StatusUnprocessableEntity,
			`{"error":"Could not find domain 'example.com.'"}`), nil
	})

	zone, err := client.GetForwardZone(context.Background(), "example.com.")
	assert.Nil(t, zone)
	assert.True(t, errors.Is(err, ErrNotFound),
		"a 422 naming a missing domain must be reported as ErrNotFound, got: %v", err)
}

// A 404 is mapped to ErrNotFound by the shared request layer rather than by the
// message match above.
func TestGetForwardZoneTreats404AsNotFound(t *testing.T) {
	client := newRecursorTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	zone, err := client.GetForwardZone(context.Background(), "example.com.")
	assert.Nil(t, zone)
	assert.True(t, errors.Is(err, ErrNotFound))
}

// A rejection that is not about a missing domain must stay an error, otherwise
// a genuine failure would be silently read as "zone absent" and Terraform would
// try to recreate a zone that is already there.
func TestGetForwardZoneOtherErrorIsNotNotFound(t *testing.T) {
	client := newRecursorTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnprocessableEntity,
			`{"error":"Unable to parse forwarder address"}`), nil
	})

	zone, err := client.GetForwardZone(context.Background(), "example.com.")
	assert.Nil(t, zone)
	if !assert.Error(t, err) {
		return
	}
	assert.False(t, errors.Is(err, ErrNotFound))
	assert.Contains(t, err.Error(), "Unable to parse forwarder address")
}

func TestGetForwardZoneSuccess(t *testing.T) {
	client := newRecursorTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK,
			`{"name":"example.com.","servers":["192.0.2.1:5300"],"recursion_desired":true}`), nil
	})

	zone, err := client.GetForwardZone(context.Background(), "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "example.com.", zone.Name)
	assert.Equal(t, []string{"192.0.2.1:5300"}, zone.Servers)
}

// ZoneExists is one of the few callers that treats a non-success status as data
// rather than as a failure, reading the status code the request layer returns
// alongside the error. Both branches of that contract are checked here.
func TestZoneExistsReportsPresence(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com.", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"id":"example.com.","name":"example.com."}`), nil
	})

	exists, err := client.ZoneExists(context.Background(), "example.com.")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestZoneExistsReportsAbsence(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	exists, err := client.ZoneExists(context.Background(), "example.com.")
	assert.NoError(t, err, "a 404 means the zone is absent, not that the lookup failed")
	assert.False(t, exists)
}

// A status outside the accepted set is still a real failure and must not be
// reported as a confident "zone does not exist".
func TestZoneExistsSurfacesUnexpectedStatus(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `{"error":"backend unavailable"}`), nil
	})

	exists, err := client.ZoneExists(context.Background(), "example.com.")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestCreateZone(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		return jsonResponse(http.StatusCreated,
			`{"id":"example.com.","name":"example.com.","kind":"Native"}`), nil
	})

	zone, err := client.CreateZone(context.Background(), ZoneInfo{Name: "example.com.", Kind: "Native"})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "example.com.", zone.Name)
	assert.Equal(t, "Native", zone.Kind)
}

// Only a 201 counts as a created zone; anything else must surface the server's
// own message rather than being decoded as success.
func TestCreateZoneRejectsUnexpectedStatus(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusConflict, `{"error":"Domain already exists"}`), nil
	})

	_, err := client.CreateZone(context.Background(), ZoneInfo{Name: "example.com."})
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "Domain already exists")
}

// UpdateZone and DeleteZone both drop the zone from the cache on success, so a
// read issued straight afterwards must go back to the server rather than being
// served the pre-write copy.
func TestUpdateZoneInvalidatesZoneCache(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		assert.Equal(t, http.MethodPut, r.Method)
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NoError(t, client.UpdateZone(ctx, "example.com.", ZoneInfoUpd{Kind: "Native"})) {
		return
	}

	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(2), atomic.LoadInt32(&listCalls),
		"read after update should refetch instead of serving the stale zone")
}

func TestDeleteZoneInvalidatesZoneCache(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		assert.Equal(t, http.MethodDelete, r.Method)
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NoError(t, client.DeleteZone(ctx, "example.com.")) {
		return
	}

	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(2), atomic.LoadInt32(&listCalls))
}

// A failed write must leave the cache alone: the zone on the server did not
// change, so dropping the entry would cost an extra refetch for nothing.
func TestFailedZoneWriteKeepsCache(t *testing.T) {
	var listCalls int32

	client := newCachingTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&listCalls, 1)
			return jsonResponse(http.StatusOK, oneRecordZone), nil
		}
		return jsonResponse(http.StatusInternalServerError, `{"error":"backend unavailable"}`), nil
	})

	ctx := context.Background()

	_, err := client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	assert.Error(t, client.UpdateZone(ctx, "example.com.", ZoneInfoUpd{Kind: "Native"}))

	_, err = client.ListRecords(ctx, "example.com.")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&listCalls),
		"a failed write should leave the cached zone in place")
}
