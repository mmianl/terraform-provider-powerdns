package powerdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourcePDNSZone_basic(t *testing.T) {
	zoneName := "example.com."

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSZoneConfig(zoneName),
				Check: resource.ComposeTestCheckFunc(
					testAccDataSourcePDNSZoneCheck("data.powerdns_zone.test", zoneName),
				),
			},
		},
	})
}

func TestAccDataSourcePDNSZone_withRecords(t *testing.T) {
	zoneName := "example.com."

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSZoneConfigWithRecords(zoneName),
				Check: resource.ComposeTestCheckFunc(
					testAccDataSourcePDNSZoneCheckWithRecords("data.powerdns_zone.test", zoneName),
				),
			},
		},
	})
}

func testAccDataSourcePDNSZoneConfig(zoneName string) string {
	return fmt.Sprintf(`
resource "powerdns_zone" "test" {
  name         = "%s"
  kind         = "Native"
  soa_edit_api = "DEFAULT"
}

data "powerdns_zone" "test" {
  name       = "%s"
  depends_on = [powerdns_zone.test]
}
`, zoneName, zoneName)
}

func testAccDataSourcePDNSZoneConfigWithRecords(zoneName string) string {
	return fmt.Sprintf(`
resource "powerdns_zone" "test" {
  name         = "%s"
  kind         = "Native"
  soa_edit_api = "DEFAULT"
}

data "powerdns_zone" "test" {
  name       = "%s"
  depends_on = [powerdns_zone.test]
}

output "zone_records" {
  value = data.powerdns_zone.test.records
}

output "a_records" {
  value = [
    for record in data.powerdns_zone.test.records :
    record
    if record.type == "A"
  ]
}
`, zoneName, zoneName)
}

func testAccDataSourcePDNSZoneCheck(dataSourceName, zoneName string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(dataSourceName, "name", zoneName),
		resource.TestCheckResourceAttrSet(dataSourceName, "kind"),
		resource.TestCheckResourceAttrSet(dataSourceName, "account"),
		resource.TestCheckResourceAttr(dataSourceName, "soa_edit_api", "DEFAULT"),
		resource.TestCheckResourceAttrSet(dataSourceName, "records.#"),
	)
}

func testAccDataSourcePDNSZoneCheckWithRecords(dataSourceName, zoneName string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(dataSourceName, "name", zoneName),
		resource.TestCheckResourceAttrSet(dataSourceName, "kind"),
		resource.TestCheckResourceAttrSet(dataSourceName, "account"),
		resource.TestCheckResourceAttr(dataSourceName, "soa_edit_api", "DEFAULT"),
		resource.TestCheckResourceAttrSet(dataSourceName, "records.#"),
		// Check that records have the expected structure
		resource.TestCheckResourceAttrSet(dataSourceName, "records.0.name"),
		resource.TestCheckResourceAttrSet(dataSourceName, "records.0.type"),
		resource.TestCheckResourceAttrSet(dataSourceName, "records.0.content"),
		resource.TestCheckResourceAttrSet(dataSourceName, "records.0.ttl"),
		resource.TestCheckResourceAttrSet(dataSourceName, "records.0.disabled"),
	)
}
