package powerdns

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(fn roundTripFunc) *PowerDNSClient {
	return &PowerDNSClient{
		BaseClient: &BaseClient{
			ServerURL:  "https://pdns.example.test",
			APIKey:     "test-key",
			APIVersion: 1,
			HTTP:       &http.Client{Transport: fn},
		},
	}
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestListZoneMetadata(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com./metadata", r.URL.Path)
		return jsonResponse(http.StatusOK, `[{"kind":"ALSO-NOTIFY","metadata":["192.0.2.10","192.0.2.11:5300"]}]`), nil
	})

	metadata, err := client.ListZoneMetadata(context.Background(), "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Len(t, metadata, 1) {
		return
	}
	assert.Equal(t, "ALSO-NOTIFY", metadata[0].Kind)
	assert.Equal(t, []string{"192.0.2.10", "192.0.2.11:5300"}, metadata[0].Metadata)
}

func TestReplaceZoneMetadata(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com./metadata/ALLOW-AXFR-FROM", r.URL.Path)

		reqBody, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return nil, err
		}
		defer func() {
			err := r.Body.Close()
			assert.NoError(t, err)
		}()

		var req ZoneMetadata
		err = json.NewDecoder(bytes.NewReader(reqBody)).Decode(&req)
		if !assert.NoError(t, err) {
			return nil, err
		}

		assert.Equal(t, "ALLOW-AXFR-FROM", req.Kind)
		assert.Equal(t, []string{"AUTO-NS", "198.51.100.0/24"}, req.Metadata)
		return jsonResponse(http.StatusOK, `{"kind":"ALLOW-AXFR-FROM","metadata":["AUTO-NS","198.51.100.0/24"]}`), nil
	})

	err := client.ReplaceZoneMetadata(context.Background(), "example.com.", "ALLOW-AXFR-FROM", []string{"AUTO-NS", "198.51.100.0/24"})
	assert.NoError(t, err)
}

func TestGetZoneMetadata(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com./metadata/ALSO-NOTIFY", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"kind":"ALSO-NOTIFY","metadata":["192.0.2.10","192.0.2.11:5300"]}`), nil
	})

	md, err := client.GetZoneMetadata(context.Background(), "example.com.", "ALSO-NOTIFY")
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "ALSO-NOTIFY", md.Kind)
	assert.Equal(t, []string{"192.0.2.10", "192.0.2.11:5300"}, md.Metadata)
}

func TestDeleteZoneMetadata(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com./metadata/ALSO-NOTIFY", r.URL.Path)
		return jsonResponse(http.StatusNoContent, ""), nil
	})

	err := client.DeleteZoneMetadata(context.Background(), "example.com.", "ALSO-NOTIFY")
	assert.NoError(t, err)
}
