package powerdns

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccPDNSViewZoneAssociationBasic(t *testing.T) {
	resourceName := "powerdns_view_zone_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckPDNSViewZoneAssociationDestroy(s)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccPDNSViewZoneAssociationConfigBasic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "view", "test-association-view"),
					resource.TestCheckResourceAttr(resourceName, "zone", "association.example."),
					testAccCheckPDNSViewContainsZone("test-association-view", "association.example."),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSViewZoneAssociationInvalidView(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPDNSViewZoneAssociationConfigInvalidView,
				ExpectError: regexp.MustCompile("must not start with a dot or a space"),
			},
		},
	})
}

func TestAccPDNSViewZoneAssociationInvalidZone(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPDNSViewZoneAssociationConfigInvalidZone,
				ExpectError: regexp.MustCompile("variant name may contain only lowercase letters, digits, underscore and dash"),
			},
		},
	})
}

func testAccCheckPDNSViewContainsZone(viewName string, zoneName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProvider.Meta().(*ProviderClients).PDNS
		view, err := client.GetView(context.Background(), viewName)
		if err != nil {
			return fmt.Errorf("error getting view %s: %w", viewName, err)
		}

		for _, zone := range view.Zones {
			if zone == zoneName {
				return nil
			}
		}

		return fmt.Errorf("view %s missing zone %s in %v", viewName, zoneName, view.Zones)
	}
}

func testAccCheckPDNSViewZoneAssociationDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_view_zone_association" {
			continue
		}

		viewName := rs.Primary.Attributes["view"]
		zoneName := rs.Primary.Attributes["zone"]
		if viewName == "" || zoneName == "" {
			continue
		}

		client := testAccProvider.Meta().(*ProviderClients).PDNS
		view, err := client.GetView(context.Background(), viewName)
		if err != nil {
			if err == ErrNotFound {
				continue
			}
			return fmt.Errorf("error getting view %s: %w", viewName, err)
		}

		for _, zone := range view.Zones {
			if zone == zoneName {
				return fmt.Errorf("view zone association still exists: view %s zone %s", viewName, zoneName)
			}
		}
	}
	return nil
}

const testAccPDNSViewZoneAssociationConfigBasic = `
resource "powerdns_zone" "test" {
  name = "association.example."
  kind = "Native"
}

resource "powerdns_view_zone_association" "test" {
  view = "test-association-view"
  zone = powerdns_zone.test.name
}`

const testAccPDNSViewZoneAssociationConfigInvalidView = `
resource "powerdns_zone" "test" {
  name = "invalid-association-view.example."
  kind = "Native"
}

resource "powerdns_view_zone_association" "test" {
  view = ".invalid-association-view"
  zone = powerdns_zone.test.name
}`

const testAccPDNSViewZoneAssociationConfigInvalidZone = `
resource "powerdns_view_zone_association" "test" {
  view = "test-association-view"
  zone = "association.example..BadVariant"
}`
