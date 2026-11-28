package powerdns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
			h := newResourceHarness(t, newRecursorForwardZoneResource(), &ProviderClients{Recursor: client})
			imported, diags := h.importState("example.com.")
			if tt.wantError != "" {
				if assert.True(t, diags.HasError(), "unsupported zone must not be imported") {
					assert.Contains(t, diags[0].Detail(), tt.wantError)
				}
				return
			}
			if !assert.False(t, diags.HasError(), "%v", diags) {
				return
			}
			assert.Equal(t, "example.com.", tfAttrString(t, imported, "zone"))
			assert.Equal(t, "Forwarded", tfAttrString(t, imported, "kind"))
			assert.Equal(t, []string{"192.0.2.1"}, tfAttrStrings(t, imported, "servers"))
		})
	}
}

func TestAccPowerDNSRecursorForwardZone_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckRecursor(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPowerDNSRecursorForwardZoneDestroy,
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
		PreCheck:                 func() { testAccPreCheckRecursor(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					// PreConfig runs before the harness configures its provider.
					// Use a fresh client so this test also works in isolation.
					cfg, err := resolveConfig(providerModel{})
					if err != nil {
						t.Fatalf("failed to resolve out-of-band configuration: %v", err)
					}
					_, recursor, err := cfg.Clients(context.Background())
					if err != nil {
						t.Fatalf("failed to configure out-of-band client: %v", err)
					}
					client := &ProviderClients{Recursor: recursor}
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

func TestSameServerAddress(t *testing.T) {
	for _, tc := range []struct {
		configured, api string
		want            bool
	}{
		{"192.0.2.1", "192.0.2.1", true},
		{"pdns:5300", "172.23.0.2:5300", true},
		{"ns.example.com", "192.0.2.9", true},
		{"pdns:5300", "172.23.0.2:5301", false},
		{"192.0.2.1", "192.0.2.2", false},
		{"pdns:5300", "other:5300", false},
	} {
		assert.Equal(t, tc.want, sameServerAddress(tc.configured, tc.api), "%s vs %s", tc.configured, tc.api)
	}
}

func TestRecursorForwardZoneCreateKeepsStateWhenReadFails(t *testing.T) {
	client := newTestRecursorClient("localhost", writesSucceedReadsFail(http.StatusCreated, "", 0, ""))
	assertCreateKeepsState(t, newRecursorForwardZoneResource(), &ProviderClients{Recursor: client},
		map[string]tftypes.Value{"zone": tfString("kept.example."), "servers": tfStringList("192.0.2.53")},
		"kept.example.")
}

// The Recursor stores an address in place of a host name. The host name written
// in the configuration stays in state, on create and on every refresh.
func TestRecursorForwardZoneKeepsConfiguredHostName(t *testing.T) {
	client := newTestRecursorClient("localhost", func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			return jsonResponse(http.StatusCreated, ""), nil
		}
		return jsonResponse(http.StatusOK,
			`{"name":"kept.example.","kind":"Forwarded","servers":["172.23.0.2:5300","192.0.2.9:53"],"recursion_desired":false}`), nil
	})
	h := newResourceHarness(t, newRecursorForwardZoneResource(), &ProviderClients{Recursor: client})
	state, diags := h.create(map[string]tftypes.Value{
		"zone": tfString("kept.example."), "servers": tfStringList("pdns:5300", "192.0.2.9"),
	})
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"pdns:5300", "192.0.2.9"}, tfAttrStrings(t, state, "servers"))

	state, diags = h.read(state)
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"pdns:5300", "192.0.2.9"}, tfAttrStrings(t, state, "servers"))

	// A change of port or of an address written as an address is not hidden. The
	// provider cannot resolve names, so a host name matches any address on its
	// port.
	other := newTestRecursorClient("localhost", func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK,
			`{"name":"kept.example.","kind":"Forwarded","servers":["172.23.0.2:5301","198.51.100.7:53"],"recursion_desired":false}`), nil
	})
	changed := newResourceHarness(t, newRecursorForwardZoneResource(), &ProviderClients{Recursor: other})
	state, diags = changed.read(state)
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"172.23.0.2:5301", "198.51.100.7"}, tfAttrStrings(t, state, "servers"))
}

func TestSameServerAddressIPv6Spellings(t *testing.T) {
	for _, tc := range []struct {
		configured, api string
		want            bool
	}{
		{"[2001:0db8:0000::1]:5300", "[2001:db8::1]:5300", true},
		{"2001:0db8:0000::1", "2001:db8::1", true},
		{"[2001:0db8:0000::1]:5300", "[2001:db8::1]:5301", false},
		{"[2001:0db8:0000::1]:5300", "[2001:db8::2]:5300", false},
	} {
		assert.Equal(t, tc.want, sameServerAddress(tc.configured, tc.api), "%s vs %s", tc.configured, tc.api)
	}
}
