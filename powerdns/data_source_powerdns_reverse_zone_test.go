package powerdns

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourcePDNSReverseZoneRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			w.WriteHeader(200)
		case "/api/v1/servers/localhost":
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		case "/api/v1/servers/localhost/zones/16.172.in-addr.arpa.":
			// Return zone with NS records in ResourceRecordSets format
			zone := ZoneInfo{
				ID:   "16.172.in-addr.arpa.",
				Name: "16.172.in-addr.arpa.",
				Kind: "Master",
				ResourceRecordSets: []ResourceRecordSet{
					{
						Name: "16.172.in-addr.arpa.",
						Type: "NS",
						TTL:  3600,
						Records: []Record{
							{Content: "ns1.example.com."},
							{Content: "ns2.example.com."},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, dataSourcePDNSReverseZone().Schema, map[string]interface{}{
		"cidr": "172.16.0.0/16",
	})

	err := dataSourcePDNSReverseZoneRead(rd, client)
	if err != nil {
		t.Fatalf("dataSourcePDNSReverseZoneRead failed: %v", err)
	}

	if rd.Id() != "16.172.in-addr.arpa." {
		t.Errorf("Expected ID '16.172.in-addr.arpa.', got '%s'", rd.Id())
	}

	name, ok := rd.GetOk("name")
	if !ok || name.(string) != "16.172.in-addr.arpa." {
		t.Errorf("Expected name '16.172.in-addr.arpa.', got '%v'", name)
	}

	kind, ok := rd.GetOk("kind")
	if !ok || kind.(string) != "Master" {
		t.Errorf("Expected kind 'Master', got '%v'", kind)
	}

	nameservers := rd.Get("nameservers").([]interface{})
	if len(nameservers) != 2 {
		t.Errorf("Expected 2 nameservers, got %d", len(nameservers))
	}
}

func TestDataSourcePDNSReverseZoneReadNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			w.WriteHeader(200)
		case "/api/v1/servers/localhost":
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		case "/api/v1/servers/localhost/zones/16.172.in-addr.arpa.":
			// Return empty zone to simulate not found
			zone := ZoneInfo{}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, dataSourcePDNSReverseZone().Schema, map[string]interface{}{
		"cidr": "172.16.0.0/16",
	})

	err := dataSourcePDNSReverseZoneRead(rd, client)
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}

func TestDataSourcePDNSReverseZoneReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			w.WriteHeader(200)
		case "/api/v1/servers/localhost":
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		case "/api/v1/servers/localhost/zones/16.172.in-addr.arpa.":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Internal Server Error"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, dataSourcePDNSReverseZone().Schema, map[string]interface{}{
		"cidr": "172.16.0.0/16",
	})

	err := dataSourcePDNSReverseZoneRead(rd, client)
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}

func TestDataSourcePDNSReverseZoneReadInvalidCIDR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			w.WriteHeader(200)
		case "/api/v1/servers/localhost":
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, dataSourcePDNSReverseZone().Schema, map[string]interface{}{
		"cidr": "invalid-cidr",
	})

	err := dataSourcePDNSReverseZoneRead(rd, client)
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}

func TestDataSourcePDNSReverseZoneReadNoNameservers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			w.WriteHeader(200)
		case "/api/v1/servers/localhost":
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		case "/api/v1/servers/localhost/zones/16.172.in-addr.arpa.":
			zone := ZoneInfo{
				ID:   "16.172.in-addr.arpa.",
				Name: "16.172.in-addr.arpa.",
				Kind: "Master",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		case "/api/v1/servers/localhost/zones/16.172.in-addr.arpa./16.172.in-addr.arpa./NS":
			// Return empty list for NS records
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode([]Record{}); err != nil {
				t.Errorf("Failed to encode records response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, dataSourcePDNSReverseZone().Schema, map[string]interface{}{
		"cidr": "172.16.0.0/16",
	})

	err := dataSourcePDNSReverseZoneRead(rd, client)
	if err != nil {
		t.Fatalf("dataSourcePDNSReverseZoneRead failed: %v", err)
	}

	nameservers := rd.Get("nameservers").([]interface{})
	if len(nameservers) != 0 {
		t.Errorf("Expected 0 nameservers, got %d", len(nameservers))
	}
}
