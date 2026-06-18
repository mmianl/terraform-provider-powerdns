package powerdns

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newMissingZonePDNSServer serves a v1 API where every zone lookup 404s,
// mimicking a zone that has already been deleted (e.g. during test teardown).
func newMissingZonePDNSServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/servers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})
	mux.HandleFunc("/api/v1/servers/localhost/zones/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "Not Found"}`))
	})

	return httptest.NewServer(mux)
}

// A missing zone means the record cannot exist; RecordExists must report
// false without surfacing the 404 as an error. CheckDestroy relies on this:
// it queries records after the zone has been torn down.
func TestRecordExists_MissingZoneIsNotError(t *testing.T) {
	ctx := context.Background()
	srv := newMissingZonePDNSServer(t)
	defer srv.Close()

	client, err := NewPowerDNSClient(ctx, srv.URL, "testapikey", &tls.Config{}, false, "0", 0)
	if err != nil {
		t.Fatalf("NewPowerDNSClient: %v", err)
	}

	exists, err := client.RecordExistsByID(ctx, "gone.example.", "host.gone.example.:::A")
	if err != nil {
		t.Fatalf("RecordExistsByID on missing zone returned error: %v", err)
	}
	if exists {
		t.Fatalf("RecordExistsByID on missing zone returned true, want false")
	}
}

// ListRecords on a missing zone returns no records and no error.
func TestListRecords_MissingZoneIsNotError(t *testing.T) {
	ctx := context.Background()
	srv := newMissingZonePDNSServer(t)
	defer srv.Close()

	client, err := NewPowerDNSClient(ctx, srv.URL, "testapikey", &tls.Config{}, false, "0", 0)
	if err != nil {
		t.Fatalf("NewPowerDNSClient: %v", err)
	}

	records, err := client.ListRecords(ctx, "gone.example.")
	if err != nil {
		t.Fatalf("ListRecords on missing zone returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("ListRecords on missing zone returned %d records, want 0", len(records))
	}
}

// GetRecordSetByID surfaces a missing zone as errZoneNotFound so the record
// Read can tell "zone gone" apart from a real API failure and clear state.
func TestGetRecordSetByID_MissingZoneIsTyped(t *testing.T) {
	ctx := context.Background()
	srv := newMissingZonePDNSServer(t)
	defer srv.Close()

	client, err := NewPowerDNSClient(ctx, srv.URL, "testapikey", &tls.Config{}, false, "0", 0)
	if err != nil {
		t.Fatalf("NewPowerDNSClient: %v", err)
	}

	_, err = client.GetRecordSetByID(ctx, "gone.example.", "host.gone.example.:::A")
	if !errors.Is(err, errZoneNotFound) {
		t.Fatalf("GetRecordSetByID on missing zone: got err %v, want errZoneNotFound", err)
	}
}
