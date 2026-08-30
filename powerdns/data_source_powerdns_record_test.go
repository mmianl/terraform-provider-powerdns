package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourcePDNSRecord_basic(t *testing.T) {
	dataSourceName := "data.powerdns_record.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheckComments(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSRecordConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "zone", "test-ds-record-sysa.xyz."),
					resource.TestCheckResourceAttr(dataSourceName, "name", "www.test-ds-record-sysa.xyz."),
					resource.TestCheckResourceAttr(dataSourceName, "type", "A"),
					resource.TestCheckResourceAttr(dataSourceName, "ttl", "300"),
					resource.TestCheckResourceAttr(dataSourceName, "disabled", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "records.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "comments.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "comments.0", "managed-by=terraform"),
					resource.TestCheckResourceAttr(dataSourceName, "comments.1", "owner=dns-team"),
				),
			},
		},
	})
}

const testAccDataSourcePDNSRecordConfig = `
resource "powerdns_zone" "test-ds-record" {
	name = "test-ds-record-sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-ds-record" {
	zone = powerdns_zone.test-ds-record.name
	name = "www.test-ds-record-sysa.xyz."
	type = "A"
	ttl = 300
	disabled = true
	records = ["192.0.2.10", "192.0.2.11"]
	comments = [
		"managed-by=terraform",
		"owner=dns-team",
	]
}

data "powerdns_record" "test" {
	zone = powerdns_zone.test-ds-record.name
	name = powerdns_record.test-ds-record.name
	type = powerdns_record.test-ds-record.type

	depends_on = [powerdns_record.test-ds-record]
}
`
