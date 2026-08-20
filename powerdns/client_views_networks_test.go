package powerdns

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetView(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/views/test-view", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"id":"test-view","name":"test-view","zones":[]}`), nil
	})

	view, err := client.GetView(context.Background(), "test-view")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "test-view", view.Name)
	assert.Empty(t, view.Zones)
}

func TestGetViewWithZoneAssociations(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/views/test-view", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"id":"test-view","name":"test-view","zones":["example.com.","example.com..internal"]}`), nil
	})

	view, err := client.GetView(context.Background(), "test-view")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "test-view", view.Name)
	assert.Equal(t, []string{"example.com.", "example.com..internal"}, view.Zones)
}

func TestGetViewNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
	})

	view, err := client.GetView(context.Background(), "missing")
	assert.Nil(t, view)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetViewErrorDecodeIncludesRootCause(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `not-json`), nil
	})

	view, err := client.GetView(context.Background(), "test-view")
	assert.Nil(t, view)
	assert.ErrorContains(t, err, "failed to decode error response")
}

func TestAddZoneToView(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/views/test-view", r.URL.Path)

		reqBody, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return nil, err
		}
		defer func() {
			err := r.Body.Close()
			assert.NoError(t, err)
		}()

		var req map[string]string
		err = json.NewDecoder(bytes.NewReader(reqBody)).Decode(&req)
		if !assert.NoError(t, err) {
			return nil, err
		}
		assert.Equal(t, "example.com.", req["name"])
		return jsonResponse(http.StatusCreated, ``), nil
	})

	err := client.AddZoneToView(context.Background(), "test-view", "example.com.")
	assert.NoError(t, err)
}

func TestAddZoneToViewBackendHint(t *testing.T) {
	tests := []struct {
		name          string
		serverMessage string
		wantHint      bool
	}{
		{
			name:          "backend failure",
			serverMessage: "Failed to add example.com. to view test-view",
			wantHint:      true,
		},
		{
			name:          "semantic failure",
			serverMessage: "Empty view names are not allowed",
			wantHint:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusUnprocessableEntity, `{"error":`+strconv.Quote(test.serverMessage)+`}`), nil
			})

			err := client.AddZoneToView(context.Background(), "test-view", "example.com.")
			if !assert.Error(t, err) {
				return
			}
			if test.wantHint {
				assert.ErrorContains(t, err, "LMDB")
			} else {
				assert.NotContains(t, err.Error(), "LMDB")
			}
		})
	}
}

func TestRemoveZoneFromView(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/views/test-view/example.com.", r.URL.Path)
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	err := client.RemoveZoneFromView(context.Background(), "test-view", "example.com.")
	assert.NoError(t, err)
}

func TestGetNetwork(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/networks/192.0.2.0/24", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"network":"192.0.2.0/24","view":"blue"}`), nil
	})

	network, err := client.GetNetwork(context.Background(), "192.0.2.0", "24")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "192.0.2.0/24", network.Network)
	assert.Equal(t, "blue", network.View)
}

func TestListNetworks(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/networks", r.URL.Path)
		return jsonResponse(http.StatusOK, `[{"network":"192.0.2.0/24","view":"blue"},{"network":"2001:db8::/32","view":"green"}]`), nil
	})

	networks, err := client.ListNetworks(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, []Network{
		{Network: "192.0.2.0/24", View: "blue"},
		{Network: "2001:db8::/32", View: "green"},
	}, networks)
}

func TestSetNetwork(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/networks/192.0.2.0/24", r.URL.Path)

		reqBody, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return nil, err
		}
		defer func() {
			err := r.Body.Close()
			assert.NoError(t, err)
		}()

		var req Network
		err = json.NewDecoder(bytes.NewReader(reqBody)).Decode(&req)
		if !assert.NoError(t, err) {
			return nil, err
		}
		assert.Equal(t, "blue", req.View)
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	err := client.SetNetwork(context.Background(), "192.0.2.0", "24", "blue")
	assert.NoError(t, err)
}

func TestSetNetworkBackendHint(t *testing.T) {
	tests := []struct {
		name          string
		serverMessage string
		wantHint      bool
	}{
		{
			name:          "backend failure",
			serverMessage: "Failed to setup view blue for network 192.0.2.0/24",
			wantHint:      true,
		},
		{
			name:          "semantic failure",
			serverMessage: "Invalid network format",
			wantHint:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusUnprocessableEntity, `{"error":`+strconv.Quote(test.serverMessage)+`}`), nil
			})

			err := client.SetNetwork(context.Background(), "192.0.2.0", "24", "blue")
			if !assert.Error(t, err) {
				return
			}
			if test.wantHint {
				assert.ErrorContains(t, err, "LMDB")
			} else {
				assert.NotContains(t, err.Error(), "LMDB")
			}
		})
	}
}

func TestDeleteNetwork(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/networks/192.0.2.0/24", r.URL.Path)

		reqBody, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return nil, err
		}
		defer func() {
			err := r.Body.Close()
			assert.NoError(t, err)
		}()

		var req Network
		err = json.NewDecoder(bytes.NewReader(reqBody)).Decode(&req)
		if !assert.NoError(t, err) {
			return nil, err
		}
		assert.Equal(t, "", req.View)
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	err := client.DeleteNetwork(context.Background(), "192.0.2.0", "24")
	assert.NoError(t, err)
}
