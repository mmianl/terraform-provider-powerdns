package powerdns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestPDNSServer creates a fake PDNS-like API
func newTestPDNSServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// Used by detectAPIVersion
	mux.HandleFunc("/api/v1/servers", func(w http.ResponseWriter, r *http.Request) {
		// Assert the API key header is passed through
		if r.Header.Get("X-API-Key") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`)) // body is ignored by detectAPIVersion
	})

	// Used by setServerVersion
	mux.HandleFunc("/api/v1/servers/localhost", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Minimal serverInfo JSON; only "version" is needed.
		_, _ = w.Write([]byte(`{"version":"4.9.0"}`))
	})

	return httptest.NewServer(mux)
}

func TestConfigClients_WithRecursorURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	authSrv := newTestPDNSServer(t)
	defer authSrv.Close()

	recursorSrv := newTestPDNSServer(t)
	defer recursorSrv.Close()

	cfg := &Config{
		ServerURL:         authSrv.URL,
		RecursorServerURL: recursorSrv.URL,
		APIKey:            "testapikey",
		InsecureHTTPS:     false,
		CacheEnable:       false,
		CacheMemorySize:   "0",
		CacheTTL:          0,
	}

	pdnsClient, recursorClient, err := cfg.Clients(ctx)
	if err != nil {
		t.Fatalf("Config.Clients returned error: %v", err)
	}

	if pdnsClient == nil {
		t.Fatalf("expected PowerDNS client to be non-nil when ServerURL is set")
	}

	if recursorClient == nil {
		t.Fatalf("expected Recursor client to be non-nil when RecursorServerURL is set")
	}

	// Sanity check: the base URLs should be sanitized versions of the test servers.
	if got, wantPrefix := pdnsClient.ServerURL, authSrv.URL; got != wantPrefix {
		t.Errorf("unexpected pdnsClient.ServerURL: got %q, want %q", got, wantPrefix)
	}

	if got, wantPrefix := recursorClient.ServerURL, recursorSrv.URL; got != wantPrefix {
		t.Errorf("unexpected recursorClient.ServerURL: got %q, want %q", got, wantPrefix)
	}
}

func TestConfigClients_WithoutRecursorURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	authSrv := newTestPDNSServer(t)
	defer authSrv.Close()

	cfg := &Config{
		ServerURL: authSrv.URL,
		APIKey:    "testapikey",
		// No RecursorServerURL -> recursor client should be nil
		InsecureHTTPS:   false,
		CacheEnable:     false,
		CacheMemorySize: "0",
		CacheTTL:        0,
	}

	pdnsClient, recursorClient, err := cfg.Clients(ctx)
	if err != nil {
		t.Fatalf("Config.Clients returned error: %v", err)
	}

	if pdnsClient == nil {
		t.Fatalf("expected PowerDNS client to be non-nil when ServerURL is set")
	}

	if recursorClient != nil {
		t.Fatalf("expected Recursor client to be nil when RecursorServerURL is empty")
	}
}
