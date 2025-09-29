package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourcePDNSZone_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSZoneConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_zone.test", "name", "example.com."),
					resource.TestCheckResourceAttr("data.powerdns_zone.test", "kind", "Master"),
					resource.TestCheckResourceAttrSet("data.powerdns_zone.test", "soa_edit_api"),
				),
			},
		},
	})
}

const testAccDataSourcePDNSZoneConfig = `
 resource "powerdns_zone" "test" {
   name = "example.com."
   kind = "Master"
 }

 data "powerdns_zone" "test" {
   name = "example.com."

   depends_on = [powerdns_zone.test]
 }
 `
