package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourcePDNSZoneCryptokeys_basic(t *testing.T) {
	dataSourceName := "data.powerdns_zone_cryptokeys.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneCryptokeysDataSourceConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "cryptokeys.#", "1"),
					resource.TestCheckResourceAttrSet(dataSourceName, "cryptokeys.0.key_id"),
					resource.TestCheckResourceAttrSet(dataSourceName, "cryptokeys.0.dnskey"),
					resource.TestCheckResourceAttr(dataSourceName, "cryptokeys.0.active", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "cryptokeys.0.algorithm", "ECDSAP256SHA256"),
				),
			},
		},
	})
}

const testPDNSZoneCryptokeysDataSourceConfig = `
resource "powerdns_zone" "cryptokeys-ds" {
	name = "cryptokeys-ds.sysa.abc."
	kind = "Native"
}

resource "powerdns_zone_cryptokey" "cryptokeys-ds" {
	zone      = powerdns_zone.cryptokeys-ds.name
	keytype   = "csk"
	algorithm = "ecdsa256"
}

data "powerdns_zone_cryptokeys" "test" {
	zone       = powerdns_zone_cryptokey.cryptokeys-ds.zone
	depends_on = [powerdns_zone_cryptokey.cryptokeys-ds]
}`
