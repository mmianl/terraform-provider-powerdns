package powerdns

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestParseNetworkCIDR(t *testing.T) {
	ip, prefix, err := parseNetworkCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "192.0.2.0" || prefix != "24" {
		t.Fatalf("unexpected result: %s/%s", ip, prefix)
	}
}

func TestAccPDNSNetworkBasic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckViews(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPDNSNetworkConfigBasic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_network.test", "network", "192.0.2.0/24"),
					resource.TestCheckResourceAttr("powerdns_network.test", "view", "test-network-view"),
				),
			},
			{
				ResourceName:      "powerdns_network.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSNetworkInvalidNetwork(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckViews(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPDNSNetworkConfigInvalidNetwork,
				ExpectError: regexp.MustCompile(`invalid\s+CIDR`),
			},
		},
	})
}

func TestAccPDNSNetworkInvalidView(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckViews(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPDNSNetworkConfigInvalidView,
				ExpectError: regexp.MustCompile("must not start with a dot or a space"),
			},
		},
	})
}

const testAccPDNSNetworkConfigBasic = `
resource "powerdns_zone" "test" {
  name = "network-test.example."
  kind = "Native"
}

resource "powerdns_view_zone_association" "test" {
  view = "test-network-view"
  zone = powerdns_zone.test.name
}

resource "powerdns_network" "test" {
  network = "192.0.2.0/24"
  view    = powerdns_view_zone_association.test.view
}`

const testAccPDNSNetworkConfigInvalidNetwork = `
resource "powerdns_network" "test" {
  network = "not-a-cidr"
  view    = "internal"
}`

const testAccPDNSNetworkConfigInvalidView = `
resource "powerdns_network" "test" {
  network = "192.0.2.0/24"
  view    = ".invalid-view"
}`

func TestNetworkCreateKeepsStateWhenReadFails(t *testing.T) {
	client := newTestClient(writesSucceedReadsFail(http.StatusNoContent, "", 0, ""))
	assertCreateKeepsState(t, newNetworkResource(), &ProviderClients{PDNS: client},
		map[string]tftypes.Value{"network": tfString("192.0.2.0/24"), "view": tfString("internal")},
		"192.0.2.0/24")
}

func TestSameCIDR(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"192.0.2.0/24", "192.0.2.0/24", true},
		{"2001:0db8:0000::/32", "2001:db8::/32", true},
		{"192.0.2.0/24", "192.0.2.0/25", false},
		{"192.0.2.0/24", "198.51.100.0/24", false},
		{"not-a-cidr", "192.0.2.0/24", false},
	} {
		if got := sameCIDR(tc.a, tc.b); got != tc.want {
			t.Errorf("sameCIDR(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// PowerDNS reports a network in its compressed form. The spelling written in
// the configuration has to stay in state, or Terraform rejects the result.
func TestNetworkKeepsConfiguredSpelling(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"network":"2001:db8::/32","view":"internal"}`), nil
		}
		return jsonResponse(http.StatusNoContent, ""), nil
	})
	h := newResourceHarness(t, newNetworkResource(), &ProviderClients{PDNS: client})
	config := map[string]tftypes.Value{"network": tfString("2001:0db8:0000::/32"), "view": tfString("internal")}

	state, diags := h.create(config)
	if diags.HasError() {
		t.Fatalf("create failed: %v", diags)
	}
	if got := tfAttrString(t, state, "network"); got != "2001:0db8:0000::/32" {
		t.Fatalf("network after create = %q, want the configured spelling", got)
	}

	refreshed, diags := h.read(state)
	if diags.HasError() {
		t.Fatalf("read failed: %v", diags)
	}
	if got := tfAttrString(t, refreshed, "network"); got != "2001:0db8:0000::/32" {
		t.Fatalf("network after refresh = %q, want the configured spelling", got)
	}
}
