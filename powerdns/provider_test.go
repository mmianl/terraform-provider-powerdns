package powerdns

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProvider *PowerDNSProvider

// testAccProtoV6ProviderFactories serves the framework provider over protocol
// version 6. All tests share one provider instance so they can reach the
// configured clients through testAccProvider.Meta().
var testAccProtoV6ProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

func init() {
	testAccProvider = New("test")().(*PowerDNSProvider)
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"powerdns": providerserver.NewProtocol6WithError(testAccProvider),
	}
}

func TestProviderSchemaValid(t *testing.T) {
	ctx := context.Background()
	resp := &provider.SchemaResponse{}
	testAccProvider.Schema(ctx, provider.SchemaRequest{}, resp)
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("provider schema invalid: %v", diags)
	}
}

func TestProviderServerIDDefault(t *testing.T) {
	t.Setenv("PDNS_SERVER_ID", "")

	cfg, err := resolveConfig(providerModel{})
	if err != nil {
		t.Fatalf("resolving config: %v", err)
	}
	if cfg.ServerID != "localhost" {
		t.Errorf("server_id default = %q, want %q", cfg.ServerID, "localhost")
	}
}

func TestProviderServerIDEnvironmentOverride(t *testing.T) {
	t.Setenv("PDNS_SERVER_ID", "zonecontrol-primary")

	cfg, err := resolveConfig(providerModel{})
	if err != nil {
		t.Fatalf("resolving config: %v", err)
	}
	if cfg.ServerID != "zonecontrol-primary" {
		t.Errorf("server_id from environment = %q, want %q", cfg.ServerID, "zonecontrol-primary")
	}
}

func TestProviderServerIDConfigWinsOverEnvironment(t *testing.T) {
	t.Setenv("PDNS_SERVER_ID", "from-env")

	cfg, err := resolveConfig(providerModel{ServerID: types.StringValue("from-config")})
	if err != nil {
		t.Fatalf("resolving config: %v", err)
	}
	if cfg.ServerID != "from-config" {
		t.Errorf("server_id = %q, want %q", cfg.ServerID, "from-config")
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
	t.Setenv("PDNS_REQUEST_TIMEOUT", "")

	cfg, err := resolveConfig(providerModel{})
	if err != nil {
		t.Fatalf("resolving config: %v", err)
	}
	if cfg.RequestTimeout != 60 {
		t.Errorf("request_timeout default = %v, want 60", cfg.RequestTimeout)
	}
}

func TestProviderRequestTimeoutEnvironmentOverride(t *testing.T) {
	t.Setenv("PDNS_REQUEST_TIMEOUT", "5")

	cfg, err := resolveConfig(providerModel{})
	if err != nil {
		t.Fatalf("resolving config: %v", err)
	}
	if cfg.RequestTimeout != 5 {
		t.Errorf("request_timeout from environment = %v, want 5", cfg.RequestTimeout)
	}
}

func TestProviderRequestTimeoutRejectsNegative(t *testing.T) {
	if _, err := resolveConfig(providerModel{RequestTimeout: types.Int64Value(-1)}); err == nil {
		t.Error("request_timeout = -1 should fail validation, got no error")
	}
}

func TestProviderBoolEnvironment(t *testing.T) {
	t.Setenv("PDNS_CACHE_REQUESTS", "true")
	t.Setenv("PDNS_INSECURE_HTTPS", "not-a-bool")

	if _, err := resolveConfig(providerModel{}); err == nil {
		t.Error("an unparsable PDNS_INSECURE_HTTPS should fail")
	}

	t.Setenv("PDNS_INSECURE_HTTPS", "")
	cfg, err := resolveConfig(providerModel{})
	if err != nil {
		t.Fatalf("resolving config: %v", err)
	}
	if !cfg.CacheEnable || cfg.InsecureHTTPS {
		t.Errorf("cache_requests=%v insecure_https=%v, want true/false", cfg.CacheEnable, cfg.InsecureHTTPS)
	}
}
