package powerdns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// Test for ID parsing functions
func TestRecordID(t *testing.T) {
	record := &Record{
		Name: "example.com",
		Type: "A",
	}

	id := record.ID()
	expected := "example.com:::A"
	assert.Equal(t, expected, id, "Record ID should be formatted correctly")
}

func TestResourceRecordSetID(t *testing.T) {
	rrSet := &ResourceRecordSet{
		Name: "test.example.com",
		Type: "AAAA",
	}

	id := rrSet.ID()
	expected := "test.example.com:::AAAA"
	assert.Equal(t, expected, id, "ResourceRecordSet ID should be formatted correctly")
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
			name:         "Valid record ID",
			recID:        "example.com:::A",
			expectedName: "example.com",
			expectedType: "A",
			expectError:  false,
		},
		{
			name:         "Valid record ID with subdomain",
			recID:        "sub.example.com:::CNAME",
			expectedName: "sub.example.com",
			expectedType: "CNAME",
			expectError:  false,
		},
		{
			name:        "Invalid record ID - wrong format",
			recID:       "example.com:A",
			expectError: true,
		},
		{
			name:        "Invalid record ID - too many parts",
			recID:       "example.com:::A:::extra",
			expectError: true,
		},
		{
			name:        "Invalid record ID - too few parts",
			recID:       "example.com",
			expectError: true,
		},
		{
			name:        "Empty record ID",
			recID:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, recType, err := parseID(tt.recID)
			if tt.expectError {
				assert.Error(t, err, "parseID should return error for invalid ID")
			} else {
				assert.NoError(t, err, "parseID should not return error for valid ID")
				assert.Equal(t, tt.expectedName, name, "Name should match expected")
				assert.Equal(t, tt.expectedType, recType, "Type should match expected")
			}
		})
	}
}

// Test for NewClient with mocked HTTP server
func TestNewClient(t *testing.T) {
	// Create a test server that responds to the version endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Request received: %s %s", r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "PowerDNS/4.8.0")

		switch r.URL.Path {
		case "/servers/localhost":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"type": "Server",
				"id": "localhost",
				"daemon_type": "authoritative",
				"version": "4.8.0",
				"url": "/api/v1/servers/localhost",
				"config_url": "/api/v1/servers/localhost/config",
				"zones_url": "/api/v1/servers/localhost/zones"
			}`))
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		case "/api/v1/servers":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`[{
				"type": "Server",
				"id": "localhost",
				"daemon_type": "authoritative",
				"url": "/api/v1/servers/localhost"
			}]`))
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		case "/api/v1/servers/localhost":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{
				"type": "Server",
				"id": "localhost",
				"daemon_type": "authoritative",
				"version": "4.8.0",
				"url": "/api/v1/servers/localhost",
				"config_url": "/api/v1/servers/localhost/config",
				"zones_url": "/api/v1/servers/localhost/zones"
			}`))
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		default:
			t.Logf("Unhandled path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name        string
		serverURL   string
		apiKey      string
		expectError bool
	}{
		{
			name:        "Valid client creation",
			serverURL:   server.URL,
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Empty server URL",
			serverURL:   "",
			apiKey:      "test-key",
			expectError: true,
		},
		{
			name:        "Empty API key",
			serverURL:   server.URL,
			apiKey:      "",
			expectError: false, // API key is not validated during client creation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := NewClient(ctx, tt.serverURL, "", tt.apiKey, nil, false, "1", 300)

			if tt.expectError {
				assert.Error(t, err, "NewClient should return error")
				assert.Nil(t, client, "Client should be nil on error")
			} else {
				assert.NoError(t, err, "NewClient should not return error")
				assert.NotNil(t, client, "Client should not be nil")
				if client != nil {
					assert.Equal(t, tt.serverURL, client.ServerURL, "ServerURL should match")
					assert.Equal(t, tt.apiKey, client.APIKey, "APIKey should match")
				}
			}
		})
	}
}
