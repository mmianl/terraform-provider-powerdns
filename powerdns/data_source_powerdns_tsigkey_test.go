package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourcePDNSTSIGKey_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyDataSourceConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_tsigkey.test", "name", "tf-datasource"),
					resource.TestCheckResourceAttr("data.powerdns_tsigkey.test", "algorithm", "hmac-sha256"),
					resource.TestCheckResourceAttr("data.powerdns_tsigkey.test", "key", testTSIGSecret),
				),
			},
		},
	})
}

const testPDNSTSIGKeyDataSourceConfig = `
resource "powerdns_tsigkey" "test-datasource" {
	name      = "tf-datasource"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecret + `"
}

data "powerdns_tsigkey" "test" {
	id = powerdns_tsigkey.test-datasource.id
}`
