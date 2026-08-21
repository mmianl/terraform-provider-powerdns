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
		serverID: "localhost",
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

func TestAuthoritativeServerIDRoutes(t *testing.T) {
	expectedPaths := []string{
		"/api/v1/servers/zonecontrol-primary/zones",
		"/api/v1/servers/zonecontrol-primary/zones/example.com./metadata",
		"/api/v1/servers/zonecontrol-primary/zones/example.com.",
		"/api/v1/servers/zonecontrol-primary/views/test-view",
		"/api/v1/servers/zonecontrol-primary/networks",
	}
	responses := []*http.Response{
		jsonResponse(http.StatusOK, `[]`),
		jsonResponse(http.StatusOK, `[]`),
		jsonResponse(http.StatusNoContent, ``),
		jsonResponse(http.StatusOK, `{"zones":[]}`),
		jsonResponse(http.StatusOK, `{"networks":[]}`),
	}
	requestIndex := 0
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if requestIndex >= len(expectedPaths) {
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
		assert.Equal(t, expectedPaths[requestIndex], r.URL.Path)
		response := responses[requestIndex]
		requestIndex++
		return response, nil
	})
	client.serverID = "zonecontrol-primary"

	_, err := client.ListZones(context.Background())
	assert.NoError(t, err)
	_, err = client.ListZoneMetadata(context.Background(), "example.com.")
	assert.NoError(t, err)
	_, err = client.ReplaceRecordSet(context.Background(), "example.com.", ResourceRecordSet{Name: "www.example.com.", Type: "A"})
	assert.NoError(t, err)
	_, err = client.GetView(context.Background(), "test-view")
	assert.NoError(t, err)
	_, err = client.ListNetworks(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(expectedPaths), requestIndex)
}

func TestServerEndpointEscapesServerID(t *testing.T) {
	client := &PowerDNSClient{serverID: "zonecontrol/primary"}
	assert.Equal(t, "/servers/zonecontrol%2Fprimary/zones", client.serverEndpoint("/zones"))
}

func TestRecursorRoutesRemainLocalhost(t *testing.T) {
	client := &RecursorClient{BaseClient: &BaseClient{
		ServerURL:  "https://pdns.example.test",
		APIKey:     "test-key",
		APIVersion: 1,
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, "/api/v1/servers/localhost/config/incoming.allow_from", r.URL.Path)
			return jsonResponse(http.StatusOK, `{"name":"incoming.allow_from","value":["192.0.2.1"]}`), nil
		})},
	}}

	_, err := client.GetConfig(context.Background(), "incoming.allow_from")
	assert.NoError(t, err)
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

func TestGetRecordSetByIDWithComments(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com.", r.URL.Path)
		assert.Equal(t, "rrsets=true", r.URL.RawQuery)
		return jsonResponse(http.StatusOK, `{
			"name":"example.com.",
			"rrsets":[
				{
					"name":"www.example.com.",
					"type":"A",
					"ttl":300,
					"records":[{"content":"192.0.2.1","disabled":false}],
					"comments":[
						{"content":"managed-by=terraform"},
						{"content":"owner=dns-team"}
					]
				}
			]
		}`), nil
	})

	rrSet, err := client.GetRecordSetByID(context.Background(), "example.com.", "www.example.com.:::A")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, rrSet) {
		return
	}

	assert.Equal(t, "www.example.com.", rrSet.Name)
	assert.Equal(t, "A", rrSet.Type)
	if assert.NotNil(t, rrSet.Comments) && assert.Len(t, *rrSet.Comments, 2) {
		assert.Equal(t, "managed-by=terraform", (*rrSet.Comments)[0].Content)
		assert.Equal(t, "owner=dns-team", (*rrSet.Comments)[1].Content)
	}
}

func TestGetZoneDoesNotRequestRRsets(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com.", r.URL.Path)
		assert.Empty(t, r.URL.RawQuery)
		return jsonResponse(http.StatusOK, `{"name":"example.com.","kind":"Native","account":"admin"}`), nil
	})

	zone, err := client.GetZone(context.Background(), "example.com.")
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "example.com.", zone.Name)
	assert.Equal(t, "Native", zone.Kind)
}

func TestListRecordsInRRSetPreservesDisabledFlag(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com.", r.URL.Path)
		assert.Equal(t, "rrsets=true", r.URL.RawQuery)
		return jsonResponse(http.StatusOK, `{
			"name":"example.com.",
			"rrsets":[
				{
					"name":"example.com.",
					"type":"SOA",
					"ttl":300,
					"records":[{"content":"ns1.example.com. hostmaster.example.com. 1 7200 600 1209600 300","disabled":true}]
				}
			]
		}`), nil
	})

	records, err := client.ListRecordsInRRSet(context.Background(), "example.com.", "example.com.", "SOA")
	if !assert.NoError(t, err) {
		return
	}

	if assert.Len(t, records, 1) {
		assert.True(t, records[0].Disabled)
		assert.Equal(t, 300, records[0].TTL)
	}
}
