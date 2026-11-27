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
