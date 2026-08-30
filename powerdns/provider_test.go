package powerdns

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

func TestProviderServerIDDefault(t *testing.T) {
	value, err := Provider().Schema["server_id"].DefaultValue()
	if err != nil {
		t.Fatalf("getting server_id default: %v", err)
	}
	if value != "localhost" {
		t.Errorf("server_id default = %q, want %q", value, "localhost")
	}
}

func TestProviderServerIDEnvironmentOverride(t *testing.T) {
	t.Setenv("PDNS_SERVER_ID", "zonecontrol-primary")

	value, err := Provider().Schema["server_id"].DefaultValue()
	if err != nil {
		t.Fatalf("getting server_id default: %v", err)
	}
	if value != "zonecontrol-primary" {
		t.Errorf("server_id from environment = %q, want %q", value, "zonecontrol-primary")
	}
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
	// A recursor is optional: the suite has to stay runnable against an
	// authoritative-only server, so this skips rather than fails.
	if v := os.Getenv("PDNS_RECURSOR_SERVER_URL"); v == "" {
		t.Skip("PDNS_RECURSOR_SERVER_URL not set; skipping recursor acceptance tests")
	}
}
