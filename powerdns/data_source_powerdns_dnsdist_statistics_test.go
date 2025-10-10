package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccPowerDNSDataSourceDNSdistStatistics_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckDNSdist(t)
		},
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSDataSourceDNSdistStatisticsConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.powerdns_dnsdist_statistics.test", "statistics.#"),
					resource.TestCheckResourceAttr("data.powerdns_dnsdist_statistics.test", "statistics.0.name", "queries"),
					resource.TestCheckResourceAttrSet("data.powerdns_dnsdist_statistics.test", "statistics.0.value"),
				),
			},
		},
	})
}

const testAccPowerDNSDataSourceDNSdistStatisticsConfig = `
data "powerdns_dnsdist_statistics" "test" {
}
`
