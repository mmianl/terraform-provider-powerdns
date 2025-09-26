package powerdns

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourcePDNSRecursorConfigCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/config/test-setting" && r.Method == "PUT" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost/config/test-setting" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode("test-value"); err != nil {
				t.Errorf("Failed to encode config value response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, server.URL, "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSRecursorConfig().Schema, map[string]interface{}{
		"name":  "test-setting",
		"value": "test-value",
	})

	err := resourcePDNSRecursorConfigCreate(rd, client)
	if err != nil {
		t.Fatalf("resourcePDNSRecursorConfigCreate failed: %v", err)
	}

	if rd.Id() != "test-setting" {
		t.Errorf("Expected ID 'test-setting', got '%s'", rd.Id())
	}
}

func TestResourcePDNSRecursorConfigRead(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		responseBody   string
		expectedValue  string
		expectError    bool
	}{
		{
			name:           "Config exists",
			serverResponse: 200,
			responseBody:   `"test-value"`,
			expectedValue:  "test-value",
			expectError:    false,
		},
		{
			name:           "Config not found",
			serverResponse: 404,
			responseBody:   `"Not found"`,
			expectedValue:  "",
			expectError:    false,
		},
		{
			name:           "Server error",
			serverResponse: 500,
			responseBody:   `"Internal Server Error"`,
			expectedValue:  "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				case "/api/v1/servers/localhost/config/test-setting":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverResponse)
					if _, err := w.Write([]byte(tt.responseBody)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client, _ := NewClient(server.URL, server.URL, "test", nil, false, "10", 300)

			rd := schema.TestResourceDataRaw(t, resourcePDNSRecursorConfig().Schema, map[string]interface{}{
				"name":  "test-setting",
				"value": "test-value",
			})
			rd.SetId("test-setting")

			err := resourcePDNSRecursorConfigRead(rd, client)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !tt.expectError && rd.Id() == "" && tt.serverResponse != 404 {
					t.Errorf("Expected ID to be set but it was empty")
				}
			}
		})
	}
}

func TestResourcePDNSRecursorConfigUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/config/test-setting" && r.Method == "PUT" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost/config/test-setting" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode("updated-value"); err != nil {
				t.Errorf("Failed to encode config value response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, server.URL, "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSRecursorConfig().Schema, map[string]interface{}{
		"name":  "test-setting",
		"value": "updated-value",
	})
	rd.SetId("test-setting")

	err := resourcePDNSRecursorConfigUpdate(rd, client)
	if err != nil {
		t.Fatalf("resourcePDNSRecursorConfigUpdate failed: %v", err)
	}
}

func TestResourcePDNSRecursorConfigDelete(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Delete success",
			serverResponse: 204,
			expectError:    false,
		},
		{
			name:           "Delete not found",
			serverResponse: 404,
			expectError:    true,
		},
		{
			name:           "Delete server error",
			serverResponse: 500,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers" {
					w.WriteHeader(200)
				} else if r.URL.Path == "/api/v1/servers/localhost" {
					serverInfo := map[string]interface{}{
						"version": "4.5.0",
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(200)
					if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
						t.Errorf("Failed to encode server info response: %v", err)
					}
				} else if r.URL.Path == "/api/v1/servers/localhost/config/test-setting" && r.Method == "DELETE" {
					w.WriteHeader(tt.serverResponse)
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client, _ := NewClient(server.URL, server.URL, "test", nil, false, "10", 300)

			rd := schema.TestResourceDataRaw(t, resourcePDNSRecursorConfig().Schema, map[string]interface{}{
				"name":  "test-setting",
				"value": "test-value",
			})
			rd.SetId("test-setting")

			err := resourcePDNSRecursorConfigDelete(rd, client)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestResourcePDNSRecursorConfigCreateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/config/error-setting" && r.Method == "PUT" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if err := json.NewEncoder(w).Encode("Internal Server Error"); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, server.URL, "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSRecursorConfig().Schema, map[string]interface{}{
		"name":  "error-setting",
		"value": "test-value",
	})

	err := resourcePDNSRecursorConfigCreate(rd, client)
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}
