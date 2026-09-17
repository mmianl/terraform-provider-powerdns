package powerdns

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "zone", "example.com."),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "servers.#", "2"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "servers.0", "192.0.2.1"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "servers.1", "192.0.2.2"),
				),
			},
			{
				ResourceName:      "powerdns_recursor_forward_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testAccPowerDNSRecursorForwardZoneConfig = `
resource "powerdns_recursor_forward_zone" "test" {
  zone    = "example.com."
  servers = ["192.0.2.1", "192.0.2.2"]
}
`

// This resource always creates zones with recursion disabled and has no
// schema field to preserve a "true" value, so importing a zone the server
// already has recursion enabled on has to fail loudly instead of reporting
// no diff and later flipping the setting on an update.
func TestAccPowerDNSRecursorForwardZone_ImportRecursiveRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheckRecursor(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					client := testAccProvider.Meta().(*ProviderClients)
					zone := &RecursorForwardZone{
						Name:             "recursive.example.com.",
						Type:             "Zone",
						Kind:             "Forwarded",
						Servers:          []string{"192.0.2.1"},
						RecursionDesired: true,
					}
					if err := client.Recursor.CreateForwardZone(context.Background(), zone); err != nil {
						t.Fatalf("failed to create recursive forward zone out of band: %v", err)
					}
				},
				Config:        testAccPowerDNSRecursorForwardZoneRecursiveImportConfig,
				ResourceName:  "powerdns_recursor_forward_zone.recursive",
				ImportState:   true,
				ImportStateId: "recursive.example.com.",
				ExpectError:   regexp.MustCompile("cannot represent"),
			},
		},
	})

	client := testAccProvider.Meta().(*ProviderClients)
	_ = client.Recursor.DeleteForwardZone(context.Background(), "recursive.example.com.")
}

const testAccPowerDNSRecursorForwardZoneRecursiveImportConfig = `
resource "powerdns_recursor_forward_zone" "recursive" {
  zone    = "recursive.example.com."
  servers = ["192.0.2.1"]
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
