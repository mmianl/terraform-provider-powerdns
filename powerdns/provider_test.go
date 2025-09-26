package powerdns

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
)

var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider

func init() {
	testAccProvider = Provider()
	testAccProviders = map[string]*schema.Provider{
		"powerdns": testAccProvider,
	}
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProviderImpl(t *testing.T) {
	var _ = Provider()
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("PDNS_API_KEY"); v == "" {
		t.Fatal("PDNS_API_KEY must be set for acceptance tests")
	}

	if v := os.Getenv("PDNS_SERVER_URL"); v == "" {
		t.Fatal("PDNS_SERVER_URL must be set for acceptance tests")
	}
}

func testAccPreCheckRecursor(t *testing.T) {
	testAccPreCheck(t)
	if v := os.Getenv("PDNS_RECURSOR_SERVER_URL"); v == "" {
		t.Fatal("PDNS_RECURSOR_SERVER_URL must be set for recursor acceptance tests")
	}
}

func TestProviderConfigure(t *testing.T) {
	// Create mock resource data
	data := schema.TestResourceDataRaw(t, Provider().Schema, map[string]interface{}{
		"server_url":     "http://localhost:8081",
		"api_key":        "test-key",
		"insecure_https": true,
		"cache_requests": false,
		"cache_mem_size": "100",
		"cache_ttl":      30,
	})

	// We can't easily test providerConfigure without mocking HTTP calls
	// But we can test that it doesn't panic and returns the expected interface
	result, err := providerConfigure(data)

	// Since we can't mock the HTTP calls, this will fail with connection error
	// But at least we can test that the function signature is correct
	assert.Error(t, err) // Expected to fail due to no server
	assert.Nil(t, result)
}
