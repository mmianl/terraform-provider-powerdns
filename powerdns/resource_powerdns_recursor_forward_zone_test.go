package powerdns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestRecursorForwardZoneImport(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "forwarded zone",
			body: `{"name":"example.com.","kind":"Forwarded","servers":["192.0.2.1:53"],"recursion_desired":false}`,
		},
		{
			name:      "native zone",
			body:      `{"name":"example.com.","kind":"Native","servers":[],"recursion_desired":false}`,
			wantError: `has kind "Native"`,
		},
		{
			name:      "unknown kind",
			body:      `{"name":"example.com.","kind":"Unknown","servers":[],"recursion_desired":false}`,
			wantError: `has kind "Unknown"`,
		},
		{
			name:      "recursive forwarded zone",
			body:      `{"name":"example.com.","kind":"Forwarded","servers":["192.0.2.1:53"],"recursion_desired":true}`,
			wantError: "recursion_desired=true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestRecursorClient("localhost", func(r *http.Request) (*http.Response, error) {
				assert.Equal(t, http.MethodGet, r.Method, "import must not change the server")
				assert.Equal(t, "/api/v1/servers/localhost/zones/example.com.", r.URL.Path)
				return jsonResponse(http.StatusOK, tt.body), nil
			})
			res := resourcePDNSRecursorForwardZone()
			data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{})
			data.SetId("example.com.")
			ctx := context.Background()
			meta := &ProviderClients{Recursor: client}
			imported, err := res.Importer.StateContext(ctx, data, meta)
			if !assert.NoError(t, err) || !assert.Len(t, imported, 1) {
				return
			}
			diags := res.ReadContext(ctx, imported[0], meta)
			if tt.wantError != "" {
				if assert.True(t, diags.HasError(), "unsupported zone must not be imported") {
					assert.Contains(t, diags[0].Summary, tt.wantError)
				}
				return
			}
			if !assert.False(t, diags.HasError(), "%v", diags) {
				return
			}
			assert.Equal(t, "example.com.", imported[0].Get("zone"))
			assert.Equal(t, "Forwarded", imported[0].Get("kind"))
			assert.Equal(t, []interface{}{"192.0.2.1"}, imported[0].Get("servers"))
		})
	}
}

func TestAccPowerDNSRecursorForwardZone_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckRecursor(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPowerDNSRecursorForwardZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSRecursorForwardZoneConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSRecursorForwardZoneExists("powerdns_recursor_forward_zone.test"),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "zone", "example.com."),
					resource.TestCheckResourceAttr("powerdns_recursor_forward_zone.test", "kind", "Forwarded"),
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
		PreCheck:          func() { testAccPreCheckRecursor(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					// PreConfig runs before the harness configures its provider.
					// Use a fresh client so this test also works in isolation.
					provider := Provider()
					data := schema.TestResourceDataRaw(t, provider.Schema, map[string]interface{}{})
					meta, diags := provider.ConfigureContextFunc(context.Background(), data)
					if diags.HasError() {
						t.Fatalf("failed to configure out-of-band client: %v", diags)
					}
					client := meta.(*ProviderClients)
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
					t.Cleanup(func() {
						if err := client.Recursor.DeleteForwardZone(context.Background(), zone.Name); err != nil && !errors.Is(err, ErrNotFound) {
							t.Errorf("failed to clean up recursive forward zone %q: %v", zone.Name, err)
						}
					})
				},
				Config:        testAccPowerDNSRecursorForwardZoneRecursiveImportConfig,
				ResourceName:  "powerdns_recursor_forward_zone.recursive",
				ImportState:   true,
				ImportStateId: "recursive.example.com.",
				ExpectError:   regexp.MustCompile("cannot represent"),
			},
		},
	})
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
