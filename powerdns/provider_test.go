package powerdns

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProvider *schema.Provider

// testAccProviderFactories is the replacement for the deprecated Providers
// field on resource.TestCase. The factory is called per test, so each one gets
// its own configured provider.
var testAccProviderFactories map[string]func() (*schema.Provider, error)

func init() {
	testAccProvider = Provider()
	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"powerdns": func() (*schema.Provider, error) {
			return testAccProvider, nil
		},
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

func TestProviderRecursorServerIDDefault(t *testing.T) {
	value, err := Provider().Schema["recursor_server_id"].DefaultValue()
	if err != nil {
		t.Fatalf("getting recursor_server_id default: %v", err)
	}
	if value != "localhost" {
		t.Errorf("recursor_server_id default = %q, want %q", value, "localhost")
	}
}

func TestProviderRecursorServerIDEnvironmentOverride(t *testing.T) {
	t.Setenv("PDNS_RECURSOR_SERVER_ID", "edge-recursor")

	value, err := Provider().Schema["recursor_server_id"].DefaultValue()
	if err != nil {
		t.Fatalf("getting recursor_server_id default: %v", err)
	}
	if value != "edge-recursor" {
		t.Errorf("recursor_server_id from environment = %q, want %q", value, "edge-recursor")
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

func TestProviderRequestTimeoutDefault(t *testing.T) {
	value, err := Provider().Schema["request_timeout"].DefaultValue()
	if err != nil {
		t.Fatalf("getting request_timeout default: %v", err)
	}
	if value != 60 {
		t.Errorf("request_timeout default = %v, want 60", value)
	}
}

func TestProviderRequestTimeoutEnvironmentOverride(t *testing.T) {
	t.Setenv("PDNS_REQUEST_TIMEOUT", "5")

	value, err := Provider().Schema["request_timeout"].DefaultValue()
	if err != nil {
		t.Fatalf("getting request_timeout default: %v", err)
	}
	// EnvDefaultFunc hands back the raw string; the SDK converts it to the
	// schema type when the value is read, so compare on the string here.
	if fmt.Sprintf("%v", value) != "5" {
		t.Errorf("request_timeout from environment = %v, want 5", value)
	}
}

func TestProviderRequestTimeoutRejectsNegative(t *testing.T) {
	_, errs := Provider().Schema["request_timeout"].ValidateFunc(-1, "request_timeout")
	if len(errs) == 0 {
		t.Error("request_timeout = -1 should fail validation, got no errors")
	}
}
