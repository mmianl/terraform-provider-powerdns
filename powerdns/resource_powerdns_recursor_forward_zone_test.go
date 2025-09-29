package powerdns

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccPowerDNSRecursorForwardZone_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheckRecursor(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPowerDNSRecursorForwardZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSRecursorForwardZoneConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSRecursorForwardZoneExists("powerdns_recursor_forward_zone.test"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "zone", "example.com"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "servers.#", "2"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "servers.0", "192.0.2.1"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "servers.1", "192.0.2.2"),
				),
			},
		},
	})
}

const testAccPowerDNSRecursorForwardZoneConfig = `
resource "powerdns_recursor_forward_zone" "test" {
  zone    = "example.com"
  servers = ["192.0.2.1", "192.0.2.2"]
}
`

func testAccCheckPowerDNSRecursorForwardZoneDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_recursor_forward_zone" {
			continue
		}

		value, err := client.GetRecursorConfigValue(context.Background(), "forward-zones")
		if err != nil {
			return err
		}

		forwardZones := parseForwardZones(value)
		if _, exists := forwardZones[rs.Primary.ID]; exists {
			return fmt.Errorf("Recursor forward zone still exists")
		}
	}

	return nil
}

func testAccCheckPowerDNSRecursorForwardZoneExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No recursor forward zone ID is set")
		}

		client := testAccProvider.Meta().(*Client)
		value, err := client.GetRecursorConfigValue(context.Background(), "forward-zones")
		if err != nil {
			return fmt.Errorf("Error getting forward-zones: %s", err)
		}

		forwardZones := parseForwardZones(value)
		if _, exists := forwardZones[rs.Primary.ID]; !exists {
			return fmt.Errorf("Recursor forward zone not found")
		}

		return nil
	}
}
