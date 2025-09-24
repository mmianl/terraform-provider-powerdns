package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccPowerDNSReverseZoneDataSource_CIDR(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSReverseZoneDataSourceConfig_CIDR,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"data.powerdns_reverse_zone.test", "name", "16.172.in-addr.arpa."),
					resource.TestCheckResourceAttr(
						"data.powerdns_reverse_zone.test", "kind", "Master"),
					resource.TestCheckResourceAttr(
						"data.powerdns_reverse_zone.test", "nameservers.0", "ns1.example.com."),
				),
			},
		},
	})
}

const testAccPowerDNSReverseZoneDataSourceConfig_CIDR = `
resource "powerdns_reverse_zone" "test" {
  cidr        = "172.16.0.0/16"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}

data "powerdns_reverse_zone" "test" {
  cidr = "172.16.0.0/16"
}
`
