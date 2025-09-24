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
