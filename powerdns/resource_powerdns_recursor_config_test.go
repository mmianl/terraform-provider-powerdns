package powerdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccPowerDNSRecursorConfig_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheckRecursor(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPowerDNSRecursorConfigDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSRecursorConfigConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSRecursorConfigExists("powerdns_recursor_config.test"),
					resource.TestCheckResourceAttr("powerdns_recursor_config.test", "name", "test-setting"),
					resource.TestCheckResourceAttr("powerdns_recursor_config.test", "value", "test-value"),
				),
			},
		},
	})
}

const testAccPowerDNSRecursorConfigConfig = `
resource "powerdns_recursor_config" "test" {
  name  = "test-setting"
  value = "test-value"
}
`

func testAccCheckPowerDNSRecursorConfigDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_recursor_config" {
			continue
		}

		_, err := client.GetRecursorConfigValue(rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Recursor config still exists")
		}
	}

	return nil
}

func testAccCheckPowerDNSRecursorConfigExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No recursor config ID is set")
		}

		client := testAccProvider.Meta().(*Client)
		_, err := client.GetRecursorConfigValue(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("Error checking if recursor config exists: %s", err)
		}

		return nil
	}
}
