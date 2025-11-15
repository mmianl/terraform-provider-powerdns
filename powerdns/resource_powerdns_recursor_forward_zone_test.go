package powerdns

import (
	"context"
	"errors"
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
	client := testAccProvider.Meta().(*ProviderClients)
	recursor := client.Recursor
	if recursor == nil {
		return fmt.Errorf("recursor client is not configured")
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_recursor_forward_zone" {
			continue
		}

		zoneName := rs.Primary.ID
		if zoneName == "" {
			continue
		}

		_, err := recursor.GetForwardZone(context.Background(), zoneName)
		if err == nil {
			return fmt.Errorf("recursor forward zone %q still exists", zoneName)
		}
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("error checking recursor forward zone %q during destroy: %w", zoneName, err)
		}
	}

	return nil
}

func testAccCheckPowerDNSRecursorForwardZoneExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no recursor forward zone ID is set")
		}

		client := testAccProvider.Meta().(*ProviderClients)
		recursor := client.Recursor
		if recursor == nil {
			return fmt.Errorf("recursor client is not configured")
		}

		zoneName := rs.Primary.ID
		_, err := recursor.GetForwardZone(context.Background(), zoneName)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("recursor forward zone %q not found", zoneName)
			}
			return fmt.Errorf("error getting recursor forward zone %q: %w", zoneName, err)
		}

		return nil
	}
}
