package powerdns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPowerDNSRecursorConfig_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckRecursor(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSRecursorConfigConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSRecursorConfigExists("powerdns_recursor_config.test"),
					resource.TestCheckResourceAttr("powerdns_recursor_config.test", "name", "allow-from"),
					resource.TestCheckResourceAttr("powerdns_recursor_config.test", "value.#", "1"),
					resource.TestCheckTypeSetElemAttr("powerdns_recursor_config.test", "value.*", "127.0.0.0/8"),
				),
			},
			{
				ResourceName:      "powerdns_recursor_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testAccPowerDNSRecursorConfigConfig = `
resource "powerdns_recursor_config" "test" {
  name  = "allow-from"
  value = ["127.0.0.0/8"]
}
`

func testAccCheckPowerDNSRecursorConfigExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no recursor config ID is set")
		}

		client := testAccProvider.Meta().(*ProviderClients)
		recursor := client.Recursor
		if recursor == nil {
			return fmt.Errorf("recursor client is not configured")
		}

		name := rs.Primary.ID
		setting, err := recursor.GetConfig(context.Background(), name)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("recursor config %q not found", name)
			}
			return fmt.Errorf("error checking if recursor config %q exists: %w", name, err)
		}

		if setting.Name != name {
			return fmt.Errorf("recursor config name mismatch: got %q, want %q", setting.Name, name)
		}

		return nil
	}
}

func TestRecursorConfigCreateKeepsStateWhenReadFails(t *testing.T) {
	client := newTestRecursorClient("localhost", writesSucceedReadsFail(http.StatusOK, "", 0, ""))
	assertCreateKeepsState(t, newRecursorConfigResource(), &ProviderClients{Recursor: client},
		map[string]tftypes.Value{"name": tfString("allow-from"), "value": tfStringSet("127.0.0.0/8")},
		"allow-from")
}
