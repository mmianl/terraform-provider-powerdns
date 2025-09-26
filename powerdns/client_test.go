package powerdns

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	freecache "github.com/coocood/freecache"
	"github.com/stretchr/testify/assert"
)

var (
	URLMissingSchemaAndNotEndingWithSlash   = "powerdnsapi.com"
	URLMissingSchemaAndEndingWithSlash      = "powerdnsapi.com/"
	URLWithSchemaAndEndingWithSlash         = "http://powerdnsapi.com/"
	URLWithSchemaAndNotEndingWithSlash      = "http://powerdnsapi.com"
	URLWithSchemaAndPath                    = "https://powerdnsapi.com/api/v2"
	URLMissingSchemaHasPort                 = "powerdnsapi.com:443"
	URLMissingSchemaHasPortAndEndsWithSlash = "powerdnsapi.com:443/"
	URLWithSchemaHasPort                    = "http://powerdnsapi.com:443"
	URLWithSchemaHasPortAndEndsWithSlash    = "http://powerdnsapi.com:443/"
)

func TestURLMissingSchema(t *testing.T) {
	url, err := sanitizeURL(URLMissingSchemaAndNotEndingWithSlash)
	assert.NoError(t, err)

	expectedURL := DefaultSchema + "://" + URLMissingSchemaAndNotEndingWithSlash
	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLMissingSchemaAndEndingWithSlash(t *testing.T) {
	url, err := sanitizeURL(URLMissingSchemaAndEndingWithSlash)
	assert.NoError(t, err)

	expectedURL := DefaultSchema + "://" +
		strings.TrimSuffix(URLMissingSchemaAndEndingWithSlash, "/")

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLWithSchemaAndEndingWithSlash(t *testing.T) {
	url, err := sanitizeURL(URLWithSchemaAndEndingWithSlash)
	assert.NoError(t, err)

	expectedURL := strings.TrimSuffix(URLWithSchemaAndEndingWithSlash, "/")

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLWithSchemaAndNotEndingWithSlash(t *testing.T) {
	url, err := sanitizeURL(URLWithSchemaAndNotEndingWithSlash)
	assert.NoError(t, err)

	expectedURL := URLWithSchemaAndNotEndingWithSlash

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLMissingSchemaHasPort(t *testing.T) {
	url, err := sanitizeURL(URLMissingSchemaHasPort)
	assert.NoError(t, err)

	expectedURL := DefaultSchema + "://" + URLMissingSchemaHasPort

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLMissingSchemaHasPortAndEndsWithSlash(t *testing.T) {
	url, err := sanitizeURL(URLMissingSchemaHasPortAndEndsWithSlash)
	assert.NoError(t, err)

	expectedURL := DefaultSchema + "://" +
		strings.TrimSuffix(URLMissingSchemaHasPortAndEndsWithSlash, "/")

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLWithSchemaHasPort(t *testing.T) {
	url, err := sanitizeURL(URLWithSchemaHasPort)
	assert.NoError(t, err)

	expectedURL := URLWithSchemaHasPort

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestURLWithSchemaHasPortAndEndsWithSlash(t *testing.T) {
	url, err := sanitizeURL(URLWithSchemaHasPortAndEndsWithSlash)
	assert.NoError(t, err)

	expectedURL := strings.TrimSuffix(URLWithSchemaHasPortAndEndsWithSlash, "/")

	assert.Equal(t, url, expectedURL,
		"Expected '"+expectedURL+"' but got '"+url+"'")
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name         string
		recID        string
		expectedName string
		expectedType string
		expectError  bool
	}{
		{
			name:         "Valid ID",
			recID:        "example.com:::A",
			expectedName: "example.com",
			expectedType: "A",
			expectError:  false,
		},
		{
			name:        "Invalid ID - too many separators",
			recID:       "example.com:::A:::extra",
			expectError: true,
		},
		{
			name:        "Invalid ID - no separator",
			recID:       "example.com",
			expectError: true,
		},
		{
			name:        "Invalid ID - empty",
			recID:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, typ, err := parseID(tt.recID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, name)
				assert.Equal(t, tt.expectedType, typ)
			}
		})
	}
}

func TestDetectAPIVersion(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expected       int
		expectError    bool
	}{
		{
			name:           "API v1 available",
			serverResponse: 200,
			expected:       1,
			expectError:    false,
		},
		{
			name:           "API v1 not available",
			serverResponse: 404,
			expected:       0,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers" {
					w.WriteHeader(tt.serverResponse)
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				ServerURL: server.URL,
				APIKey:    "test",
				HTTP:      server.Client(),
			}

			version, err := client.detectAPIVersion()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, version)
			}
		})
	}
}

func TestListZones(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones" {
			zones := []ZoneInfo{
				{ID: "1", Name: "example.com."},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zones)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	zones, err := client.ListZones()
	assert.NoError(t, err)
	assert.Len(t, zones, 1)
	assert.Equal(t, "example.com.", zones[0].Name)
}

func TestGetZone(t *testing.T) {
	tests := []struct {
		name           string
		zoneName       string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Zone exists",
			zoneName:       "example.com.",
			serverResponse: 200,
			expectError:    false,
		},
		{
			name:           "Zone does not exist",
			zoneName:       "nonexistent.com.",
			serverResponse: 404,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == `/api/v1/servers/localhost/zones/`+tt.zoneName {
					if tt.serverResponse == 200 {
						zone := ZoneInfo{
							ID:   "1",
							Name: tt.zoneName,
							Kind: "Native",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(200)
						err := json.NewEncoder(w).Encode(zone)
						assert.NoError(t, err)
					} else {
						w.WriteHeader(tt.serverResponse)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				ServerURL:  server.URL,
				APIKey:     "test",
				HTTP:       server.Client(),
				APIVersion: 1,
			}

			zone, err := client.GetZone(tt.zoneName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.serverResponse == 200 {
					// Only check Name and Kind when zone exists
					assert.Equal(t, tt.zoneName, zone.Name)
					assert.Equal(t, "Native", zone.Kind)
				}
			}
		})
	}
}

func TestZoneExists(t *testing.T) {
	tests := []struct {
		name           string
		zoneName       string
		serverResponse int
		expected       bool
		expectError    bool
	}{
		{
			name:           "Zone exists",
			zoneName:       "example.com.",
			serverResponse: 200,
			expected:       true,
			expectError:    false,
		},
		{
			name:           "Zone does not exist",
			zoneName:       "nonexistent.com.",
			serverResponse: 404,
			expected:       false,
			expectError:    false,
		},
		{
			name:           "Zone exists error",
			zoneName:       "error.com.",
			serverResponse: 500,
			expected:       false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/zones/"+tt.zoneName {
					w.WriteHeader(tt.serverResponse)
					if tt.serverResponse == 500 {
						w.Header().Set("Content-Type", "application/json")
						err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Internal Server Error"})
						assert.NoError(t, err)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				ServerURL:  server.URL,
				APIKey:     "test",
				HTTP:       server.Client(),
				APIVersion: 1,
			}

			exists, err := client.ZoneExists(tt.zoneName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, exists)
			}
		})
	}
}

func TestCreateZone(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Create zone success",
			serverResponse: 201,
			expectError:    false,
		},
		{
			name:           "Create zone error",
			serverResponse: 400,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/zones" && r.Method == "POST" {
					if tt.serverResponse == 201 {
						createdZone := ZoneInfo{
							ID:   "1",
							Name: "example.com.",
							Kind: "Native",
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(201)
						err := json.NewEncoder(w).Encode(createdZone)
						assert.NoError(t, err)
					} else {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(400)
						err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Bad Request"})
						assert.NoError(t, err)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				ServerURL:  server.URL,
				APIKey:     "test",
				HTTP:       server.Client(),
				APIVersion: 1,
			}

			zoneInfo := ZoneInfo{
				Name:        "example.com.",
				Kind:        "Native",
				Nameservers: []string{"ns1.example.com.", "ns2.example.com."},
			}

			created, err := client.CreateZone(zoneInfo)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "example.com.", created.Name)
				assert.Equal(t, "Native", created.Kind)
			}
		})
	}
}

func TestUpdateZone(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Update zone success",
			serverResponse: 204,
			expectError:    false,
		},
		{
			name:           "Update zone error",
			serverResponse: 400,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PUT" {
					if tt.serverResponse == 204 {
						w.WriteHeader(204)
					} else {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(400)
						err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Bad Request"})
						assert.NoError(t, err)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				ServerURL:  server.URL,
				APIKey:     "test",
				HTTP:       server.Client(),
				APIVersion: 1,
			}

			zoneInfoUpd := ZoneInfoUpd{
				Name:    "example.com.",
				Kind:    "Master",
				Account: "admin",
			}

			err := client.UpdateZone("example.com.", zoneInfoUpd)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteZone(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Delete zone success",
			serverResponse: 204,
			expectError:    false,
		},
		{
			name:           "Delete zone error",
			serverResponse: 500,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "DELETE" {
					if tt.serverResponse == 204 {
						w.WriteHeader(204)
					} else {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(500)
						err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Internal Server Error"})
						assert.NoError(t, err)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				ServerURL:   server.URL,
				APIKey:      "test",
				HTTP:        server.Client(),
				APIVersion:  1,
				CacheEnable: true,
				Cache:       freecache.NewCache(1), // Very small cache to force set error
				CacheTTL:    300,
			}

			err := client.DeleteZone("example.com.")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListRecordsInRRSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4"},
					{Name: "www.example.com.", Type: "A", Content: "5.6.7.8"},
					{Name: "mail.example.com.", Type: "A", Content: "9.10.11.12"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zone)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecordsInRRSet("example.com.", "www.example.com.", "A")
	assert.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, "www.example.com.", records[0].Name)
	assert.Equal(t, "A", records[0].Type)
}

func TestRecordExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4"},
					{Name: "mail.example.com.", Type: "A", Content: "9.10.11.12"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zone)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	exists, err := client.RecordExists("example.com.", "www.example.com.", "A")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = client.RecordExists("example.com.", "nonexistent.example.com.", "A")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestReplaceRecordSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	rrSet := ResourceRecordSet{
		Name:    "www.example.com.",
		Type:    "A",
		TTL:     300,
		Records: []Record{{Content: "1.2.3.4"}},
	}

	id, err := client.ReplaceRecordSet("example.com.", rrSet)
	assert.NoError(t, err)
	assert.Equal(t, "www.example.com.:::A", id)
}

func TestDeleteRecordSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.DeleteRecordSet("example.com.", "www.example.com.", "A")
	assert.NoError(t, err)
}

func TestReplaceRecordSetError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	rrSet := ResourceRecordSet{
		Name:    "www.example.com.",
		Type:    "A",
		TTL:     300,
		Records: []Record{{Content: "1.2.3.4"}},
	}

	id, err := client.ReplaceRecordSet("example.com.", rrSet)
	assert.Error(t, err)
	assert.Equal(t, "", id)
}

func TestDeleteRecordSetError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.DeleteRecordSet("example.com.", "www.example.com.", "A")
	assert.Error(t, err)
}

func TestSetServerVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := serverInfo{
				Version: "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(serverInfo)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.setServerVersion()
	assert.NoError(t, err)
	assert.Equal(t, "4.5.0", client.ServerVersion)
}

func TestSetServerVersionHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost" {
			w.Header().Set("Server", "PowerDNS/4.6.0")
			w.WriteHeader(200)
			// No body
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.setServerVersion()
	assert.NoError(t, err)
	assert.Equal(t, "4.6.0", client.ServerVersion)
}

func TestListRecordsByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4"},
					{Name: "mail.example.com.", Type: "A", Content: "9.10.11.12"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zone)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecordsByID("example.com.", "www.example.com.:::A")
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "www.example.com.", records[0].Name)
	assert.Equal(t, "A", records[0].Type)
}

func TestRecordExistsByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4"},
					{Name: "mail.example.com.", Type: "A", Content: "9.10.11.12"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zone)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	exists, err := client.RecordExistsByID("example.com.", "www.example.com.:::A")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = client.RecordExistsByID("example.com.", "nonexistent.example.com.:::A")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteRecordSetByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.DeleteRecordSetByID("example.com.", "www.example.com.:::A")
	assert.NoError(t, err)
}

func TestRecordID(t *testing.T) {
	record := Record{Name: "www.example.com.", Type: "A"}
	expected := "www.example.com.:::A"
	assert.Equal(t, expected, record.ID())
}

func TestResourceRecordSetID(t *testing.T) {
	rrSet := ResourceRecordSet{Name: "www.example.com.", Type: "A"}
	expected := "www.example.com.:::A"
	assert.Equal(t, expected, rrSet.ID())
}

func TestGetZoneInfoFromCache(t *testing.T) {
	// Test with cache disabled
	client := &Client{
		CacheEnable: false,
	}
	zoneInfo, err := client.GetZoneInfoFromCache("example.com.")
	assert.NoError(t, err)
	assert.Nil(t, zoneInfo)

	// Test with cache enabled but no cached data
	client = &Client{
		CacheEnable: true,
		Cache:       freecache.NewCache(1024 * 1024),
	}
	zoneInfo, err = client.GetZoneInfoFromCache("example.com.")
	assert.Error(t, err) // Should error because no cached data
	assert.Nil(t, zoneInfo)

	// Test with cache enabled and cached data
	expectedZoneInfo := &ZoneInfo{
		Name: "example.com.",
		Kind: "Native",
	}
	cacheValue, _ := json.Marshal(expectedZoneInfo)
	err = client.Cache.Set([]byte("example.com."), cacheValue, 0)
	assert.NoError(t, err)
	zoneInfo, err = client.GetZoneInfoFromCache("example.com.")
	assert.NoError(t, err)
	assert.NotNil(t, zoneInfo)
	assert.Equal(t, "example.com.", zoneInfo.Name)
	assert.Equal(t, "Native", zoneInfo.Kind)
}

func TestListRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4", TTL: 300},
					{Name: "mail.example.com.", Type: "A", Content: "9.10.11.12", TTL: 300},
				},
				ResourceRecordSets: []ResourceRecordSet{
					{
						Name: "api.example.com.",
						Type: "A",
						TTL:  600,
						Records: []Record{
							{Content: "5.6.7.8"},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zone)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecords("example.com.")
	assert.NoError(t, err)
	assert.Len(t, records, 3)
	assert.Equal(t, "www.example.com.", records[0].Name)
	assert.Equal(t, "A", records[0].Type)
	assert.Equal(t, "1.2.3.4", records[0].Content)
	assert.Equal(t, 300, records[0].TTL)
	assert.Equal(t, "mail.example.com.", records[1].Name)
	assert.Equal(t, "api.example.com.", records[2].Name)
	assert.Equal(t, "5.6.7.8", records[2].Content)
	assert.Equal(t, 600, records[2].TTL)
}

func TestListRecordsWithCache(t *testing.T) {
	client := &Client{
		CacheEnable: true,
		Cache:       freecache.NewCache(1024 * 1024),
		APIVersion:  1,
	}

	// Set cached zone data
	zoneInfo := &ZoneInfo{
		Name: "example.com.",
		Records: []Record{
			{Name: "cached.example.com.", Type: "A", Content: "1.2.3.4", TTL: 300},
		},
		ResourceRecordSets: []ResourceRecordSet{
			{
				Name: "api.example.com.",
				Type: "A",
				TTL:  600,
				Records: []Record{
					{Content: "5.6.7.8"},
				},
			},
		},
	}
	cacheValue, _ := json.Marshal(zoneInfo)
	err := client.Cache.Set([]byte("example.com."), cacheValue, 0)
	assert.NoError(t, err)

	records, err := client.ListRecords("example.com.")
	assert.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, "cached.example.com.", records[0].Name)
	assert.Equal(t, "api.example.com.", records[1].Name)
}

func TestListRecordsCacheError(t *testing.T) {
	client := &Client{
		CacheEnable: true,
		Cache:       freecache.NewCache(1024 * 1024),
		APIVersion:  1,
	}

	// Set corrupted cached data
	err := client.Cache.Set([]byte("example.com."), []byte("invalid json"), 0)
	assert.NoError(t, err)

	// Since cache is corrupted, it should fetch from server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4", TTL: 300},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			err := json.NewEncoder(w).Encode(zone)
			assert.NoError(t, err)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client.ServerURL = server.URL
	client.APIKey = "test"
	client.HTTP = server.Client()

	records, err := client.ListRecords("example.com.")
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "www.example.com.", records[0].Name)
}

func TestListRecordsFetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == `/api/v1/servers/localhost/zones/example.com.` {
			w.WriteHeader(404) // Zone not found
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecords("example.com.")
	assert.Error(t, err)
	assert.Nil(t, records)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		serverURL   string
		apiKey      string
		configTLS   *tls.Config
		cacheEnable bool
		cacheSizeMB string
		cacheTTL    int
		expectError bool
	}{
		{
			name:        "Valid client creation",
			serverURL:   "http://powerdns.example.com",
			apiKey:      "testkey",
			configTLS:   nil,
			cacheEnable: false,
			cacheSizeMB: "10",
			cacheTTL:    300,
			expectError: false,
		},
		{
			name:        "Invalid URL",
			serverURL:   "",
			apiKey:      "testkey",
			configTLS:   nil,
			cacheEnable: false,
			cacheSizeMB: "10",
			cacheTTL:    300,
			expectError: true,
		},
		{
			name:        "Invalid cache size",
			serverURL:   "http://powerdns.example.com",
			apiKey:      "testkey",
			configTLS:   nil,
			cacheEnable: true,
			cacheSizeMB: "invalid",
			cacheTTL:    300,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server for setServerVersion
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/servers":
					w.WriteHeader(200)
				case "/api/v1/servers/localhost":
					serverInfo := serverInfo{
						Version: "4.5.0",
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(200)
					err := json.NewEncoder(w).Encode(serverInfo)
					assert.NoError(t, err)
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			// Temporarily set the serverURL to the test server if not expecting error
			testServerURL := tt.serverURL
			if !tt.expectError {
				testServerURL = server.URL
			}

			client, err := NewClient(testServerURL, "", tt.apiKey, tt.configTLS, tt.cacheEnable, tt.cacheSizeMB, tt.cacheTTL)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, server.URL, client.ServerURL)
				assert.Equal(t, tt.apiKey, client.APIKey)
				assert.Equal(t, tt.cacheEnable, client.CacheEnable)
				assert.Equal(t, tt.cacheTTL, client.CacheTTL)
				assert.Equal(t, "4.5.0", client.ServerVersion)
			}
		})
	}
}

// Additional comprehensive tests for moderately covered functions

func TestListZonesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Internal Server Error"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	zones, err := client.ListZones()
	assert.Error(t, err)
	assert.Nil(t, zones)
}

func TestListZonesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones" {
			zones := []ZoneInfo{}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zones); err != nil {
				t.Errorf("Failed to encode zones response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	zones, err := client.ListZones()
	assert.NoError(t, err)
	assert.Len(t, zones, 0)
}

func TestGetZoneWithResourceRecordSets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				ID:   "example.com.",
				Name: "example.com.",
				Kind: "Master",
				ResourceRecordSets: []ResourceRecordSet{
					{
						Name: "www.example.com.",
						Type: "A",
						TTL:  300,
						Records: []Record{
							{Content: "1.2.3.4"},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	zone, err := client.GetZone("example.com.")
	assert.NoError(t, err)
	assert.Equal(t, "example.com.", zone.Name)
	assert.Equal(t, "Master", zone.Kind)
}

func TestCreateZoneWithAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones" && r.Method == "POST" {
			var zone ZoneInfo
			if err := json.NewDecoder(r.Body).Decode(&zone); err != nil {
				t.Errorf("Failed to decode zone request: %v", err)
			}
			assert.Equal(t, "test-account", zone.Account)

			createdZone := ZoneInfo{
				ID:      "1",
				Name:    "example.com.",
				Kind:    "Master",
				Account: "test-account",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			if err := json.NewEncoder(w).Encode(createdZone); err != nil {
				t.Errorf("Failed to encode created zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	zoneInfo := ZoneInfo{
		Name:    "example.com.",
		Kind:    "Master",
		Account: "test-account",
	}

	created, err := client.CreateZone(zoneInfo)
	assert.NoError(t, err)
	assert.Equal(t, "test-account", created.Account)
}

func TestUpdateZoneWithSOAEditAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PUT" {
			var zone ZoneInfoUpd
			if err := json.NewDecoder(r.Body).Decode(&zone); err != nil {
				t.Errorf("Failed to decode zone update request: %v", err)
			}
			assert.Equal(t, "INCEPTION-INCREMENT", zone.SoaEditAPI)

			w.WriteHeader(204)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	zoneInfoUpd := ZoneInfoUpd{
		Name:       "example.com.",
		Kind:       "Master",
		SoaEditAPI: "INCEPTION-INCREMENT",
	}

	err := client.UpdateZone("example.com.", zoneInfoUpd)
	assert.NoError(t, err)
}

func TestDeleteZoneNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "DELETE" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Zone not found"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.DeleteZone("example.com.")
	assert.Error(t, err)
}

func TestListRecordsInRRSetNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecordsInRRSet("example.com.", "nonexistent.example.com.", "A")
	assert.NoError(t, err)
	assert.Len(t, records, 0)
}

func TestRecordExistsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Internal Server Error"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	exists, err := client.RecordExists("example.com.", "www.example.com.", "A")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestReplaceRecordSetWithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(422)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Validation failed"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	rrSet := ResourceRecordSet{
		Name:    "www.example.com.",
		Type:    "A",
		TTL:     300,
		Records: []Record{{Content: "1.2.3.4"}},
	}

	id, err := client.ReplaceRecordSet("example.com.", rrSet)
	assert.Error(t, err)
	assert.Equal(t, "", id)
}

func TestDeleteRecordSetNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PATCH" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Record not found"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.DeleteRecordSet("example.com.", "www.example.com.", "A")
	assert.Error(t, err)
}

func TestSetServerVersionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Server error"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.setServerVersion()
	assert.Error(t, err)
}

func TestSetServerVersionInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if _, err := w.Write([]byte("invalid json")); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	err := client.setServerVersion()
	assert.Error(t, err)
}

func TestListRecordsByIDNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecordsByID("example.com.", "nonexistent.example.com.:::A")
	assert.NoError(t, err)
	assert.Len(t, records, 0)
}

func TestRecordExistsByIDInvalidFormat(t *testing.T) {
	client := &Client{
		ServerURL:  "http://test.com",
		APIKey:     "test",
		APIVersion: 1,
	}

	exists, err := client.RecordExistsByID("example.com.", "invalid-id-format")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestDeleteRecordSetByIDInvalidFormat(t *testing.T) {
	client := &Client{
		ServerURL:  "http://test.com",
		APIKey:     "test",
		APIVersion: 1,
	}

	err := client.DeleteRecordSetByID("example.com.", "invalid-id-format")
	assert.Error(t, err)
}

func TestGetZoneInfoFromCacheCorruptedData(t *testing.T) {
	client := &Client{
		CacheEnable: true,
		Cache:       freecache.NewCache(1024 * 1024),
	}

	// Set corrupted JSON data
	err := client.Cache.Set([]byte("example.com."), []byte("invalid json"), 0)
	assert.NoError(t, err)

	zoneInfo, err := client.GetZoneInfoFromCache("example.com.")
	assert.Error(t, err)
	assert.Nil(t, zoneInfo)
}

func TestListRecordsWithEmptyZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				Name:               "example.com.",
				Records:            []Record{},
				ResourceRecordSets: []ResourceRecordSet{},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := &Client{
		ServerURL:  server.URL,
		APIKey:     "test",
		HTTP:       server.Client(),
		APIVersion: 1,
	}

	records, err := client.ListRecords("example.com.")
	assert.NoError(t, err)
	assert.Len(t, records, 0)
}

func TestListRecordsWithCacheCorruptedData(t *testing.T) {
	client := &Client{
		CacheEnable: true,
		Cache:       freecache.NewCache(1024 * 1024),
		APIVersion:  1,
	}

	// Set corrupted cached data
	err := client.Cache.Set([]byte("example.com."), []byte("invalid json"), 0)
	assert.NoError(t, err)

	// Since cache is corrupted, it should fetch from server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				Name: "example.com.",
				Records: []Record{
					{Name: "www.example.com.", Type: "A", Content: "1.2.3.4", TTL: 300},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client.ServerURL = server.URL
	client.APIKey = "test"
	client.HTTP = server.Client()

	records, err := client.ListRecords("example.com.")
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "www.example.com.", records[0].Name)
}

// Test for GetRecursorConfig function
func TestGetRecursorConfig(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		responseBody   string
		expected       map[string]string
		expectError    bool
	}{
		{
			name:           "Get recursor config success",
			serverResponse: 200,
			responseBody:   `{"max-cache-ttl": "3600", "max-packetcache-ttl": "300"}`,
			expected:       map[string]string{"max-cache-ttl": "3600", "max-packetcache-ttl": "300"},
			expectError:    false,
		},
		{
			name:           "Get recursor config not found",
			serverResponse: 404,
			responseBody:   `{"error": "Not found"}`,
			expected:       nil,
			expectError:    true,
		},
		{
			name:           "Get recursor config server error",
			serverResponse: 500,
			responseBody:   `{"error": "Internal Server Error"}`,
			expected:       nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/config" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverResponse)
					if _, err := w.Write([]byte(tt.responseBody)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				RecursorServerURL: server.URL,
				APIKey:            "test",
				HTTP:              server.Client(),
				APIVersion:        1,
			}

			config, err := client.GetRecursorConfig()
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.expected, config)
			}
		})
	}
}

// Test for reverse zone resource schema
func TestResourcePDNSReverseZoneSchema(t *testing.T) {
	resource := resourcePDNSReverseZone()

	// Test that the resource schema is properly configured
	assert.NotNil(t, resource)
	assert.NotNil(t, resource.Schema)

	// Test required fields
	schema := resource.Schema
	assert.NotNil(t, schema["name"])
	assert.NotNil(t, schema["kind"])

	// Test field properties - just check they exist and have some basic properties
	nameSchema := schema["name"]
	assert.NotNil(t, nameSchema)

	kindSchema := schema["kind"]
	assert.NotNil(t, kindSchema)
}

// Test for GetRecursorConfigValue function
func TestGetRecursorConfigValue(t *testing.T) {
	tests := []struct {
		name           string
		configName     string
		serverResponse int
		responseBody   string
		expected       string
		expectError    bool
	}{
		{
			name:           "Get recursor config value success",
			configName:     "max-cache-ttl",
			serverResponse: 200,
			responseBody:   `"3600"`,
			expected:       "3600",
			expectError:    false,
		},
		{
			name:           "Get recursor config value not found",
			configName:     "nonexistent-config",
			serverResponse: 404,
			responseBody:   `{"error": "Not found"}`,
			expected:       "",
			expectError:    true,
		},
		{
			name:           "Get recursor config value server error",
			configName:     "max-cache-ttl",
			serverResponse: 500,
			responseBody:   `{"error": "Internal Server Error"}`,
			expected:       "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/config/"+tt.configName {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverResponse)
					if _, err := w.Write([]byte(tt.responseBody)); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				RecursorServerURL: server.URL,
				APIKey:            "test",
				HTTP:              server.Client(),
				APIVersion:        1,
			}

			value, err := client.GetRecursorConfigValue(tt.configName)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, "", value)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

// Test for SetRecursorConfigValue function
func TestSetRecursorConfigValue(t *testing.T) {
	tests := []struct {
		name           string
		configName     string
		configValue    string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Set recursor config value success",
			configName:     "max-cache-ttl",
			configValue:    "3600",
			serverResponse: 200,
			expectError:    false,
		},
		{
			name:           "Set recursor config value server error",
			configName:     "max-cache-ttl",
			configValue:    "3600",
			serverResponse: 500,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/config/"+tt.configName && r.Method == "PUT" {
					w.WriteHeader(tt.serverResponse)
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				RecursorServerURL: server.URL,
				APIKey:            "test",
				HTTP:              server.Client(),
				APIVersion:        1,
			}

			err := client.SetRecursorConfigValue(tt.configName, tt.configValue)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test for DeleteRecursorConfigValue function
func TestDeleteRecursorConfigValue(t *testing.T) {
	tests := []struct {
		name           string
		configName     string
		serverResponse int
		expectError    bool
	}{
		{
			name:           "Delete recursor config value success",
			configName:     "max-cache-ttl",
			serverResponse: 204,
			expectError:    false,
		},
		{
			name:           "Delete recursor config value not found",
			configName:     "nonexistent-config",
			serverResponse: 404,
			expectError:    true,
		},
		{
			name:           "Delete recursor config value server error",
			configName:     "max-cache-ttl",
			serverResponse: 500,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/servers/localhost/config/"+tt.configName && r.Method == "DELETE" {
					w.WriteHeader(tt.serverResponse)
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := &Client{
				RecursorServerURL: server.URL,
				APIKey:            "test",
				HTTP:              server.Client(),
				APIVersion:        1,
			}

			err := client.DeleteRecursorConfigValue(tt.configName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
