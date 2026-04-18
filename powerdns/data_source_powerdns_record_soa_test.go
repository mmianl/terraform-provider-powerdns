package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourcePDNSRecordSOA_basic(t *testing.T) {
	dataSourceName := "data.powerdns_record_soa.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSRecordSOAConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "zone", "test-ds-soa-sysa.xyz."),
					resource.TestCheckResourceAttr(dataSourceName, "name", "test-ds-soa-sysa.xyz."),
					resource.TestCheckResourceAttrSet(dataSourceName, "ttl"),
					resource.TestCheckResourceAttr(dataSourceName, "disabled", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "mname", "ns1.sysa.xyz."),
					resource.TestCheckResourceAttr(dataSourceName, "rname", "hostmaster.sysa.xyz."),
					resource.TestCheckResourceAttrSet(dataSourceName, "serial"),
					resource.TestCheckResourceAttr(dataSourceName, "refresh", "7200"),
					resource.TestCheckResourceAttr(dataSourceName, "retry", "600"),
					resource.TestCheckResourceAttr(dataSourceName, "expire", "1209600"),
					resource.TestCheckResourceAttr(dataSourceName, "minimum", "6400"),
				),
			},
		},
	})
}

const testAccDataSourcePDNSRecordSOAConfig = `
resource "powerdns_zone" "test-ds-soa" {
	name        = "test-ds-soa-sysa.xyz."
	kind        = "MASTER"
}

resource "powerdns_record_soa" "test-ds-soa" {
	zone    = powerdns_zone.test-ds-soa.name
	name    = "test-ds-soa-sysa.xyz."
	ttl     = 3600
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	serial  = 0
	refresh = 7200
	retry   = 600
	expire  = 1209600
	minimum = 6400
}

data "powerdns_record_soa" "test" {
	zone = powerdns_zone.test-ds-soa.name
	name = "test-ds-soa-sysa.xyz."

	depends_on = [powerdns_record_soa.test-ds-soa]
}
`
