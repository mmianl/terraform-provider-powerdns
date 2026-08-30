package powerdns

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
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
		PreCheck:  func() { testAccPreCheckViews(t) },
		Providers: testAccProviders,
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
		PreCheck:  func() { testAccPreCheckViews(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testAccPDNSNetworkConfigInvalidNetwork,
				ExpectError: regexp.MustCompile("invalid CIDR"),
			},
		},
	})
}

func TestAccPDNSNetworkInvalidView(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheckViews(t) },
		Providers: testAccProviders,
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
