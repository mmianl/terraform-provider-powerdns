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

func TestCryptokeyRoleChangesRequireReplacement(t *testing.T) {
	for _, roles := range []struct{ requested, reported string }{
		{"csk", "csk"}, {"ksk", "csk"}, {"zsk", "csk"},
		{"csk", "ksk"}, {"ksk", "ksk"}, {"zsk", "zsk"},
	} {
		oldRole := roles.requested
		for _, newRole := range []string{"csk", "ksk", "zsk"} {
			t.Run(oldRole+"_reported_as_"+roles.reported+"_to_"+newRole, func(t *testing.T) {
				res := resourcePDNSZoneCryptokey()
				data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
					"zone": "example.com.", "keytype": oldRole, "algorithm": "ecdsa256",
				})
				data.SetId("example.com.:1")
				client := newTestClient(func(r *http.Request) (*http.Response, error) {
					assert.Equal(t, http.MethodGet, r.Method)
					return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":1,"keytype":%q,"algorithm":"ECDSAP256SHA256","bits":256,"active":true,"published":true}`, roles.reported)), nil
				})
				ctx := context.Background()
				state := data.State()
				// Refresh twice to ensure the requested role survives in saved state.
				for i := 0; i < 2; i++ {
					refreshed, diags := res.RefreshWithoutUpgrade(ctx, state, &ProviderClients{PDNS: client})
					if !assert.False(t, diags.HasError(), "%v", diags) || !assert.NotNil(t, refreshed) {
						return
					}
					state = refreshed
					assert.Equal(t, oldRole, state.Attributes["keytype"])
				}
				diff, err := res.Diff(ctx, state, terraform.NewResourceConfigRaw(map[string]interface{}{
					"zone": "example.com.", "keytype": newRole, "algorithm": "ecdsa256",
				}), nil)
				if !assert.NoError(t, err) {
					return
				}
				if oldRole == newRole {
					assert.True(t, diff == nil || diff.Empty(), "unchanged role must produce an empty plan")
				} else if assert.NotNil(t, diff, "a role change must produce a diff") {
					assert.True(t, diff.RequiresNew(), "a role change must replace the key")
					if change := diff.Attributes["keytype"]; assert.NotNil(t, change) {
						assert.Equal(t, oldRole, change.Old)
						assert.Equal(t, newRole, change.New)
						assert.True(t, change.RequiresNew)
					}
				}
			})
		}
	}
}

func TestCryptokeyImportUsesServerRole(t *testing.T) {
	for _, role := range []string{"csk", "ksk", "zsk"} {
		t.Run(role, func(t *testing.T) {
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":1,"keytype":%q,"algorithm":"ECDSAP256SHA256","bits":256,"active":true,"published":true}`, role)), nil
			})
			res := resourcePDNSZoneCryptokey()
			data := res.Data(nil)
			data.SetId("example.com.:1")
			imported, err := res.Importer.StateContext(context.Background(), data, &ProviderClients{PDNS: client})
			if assert.NoError(t, err) && assert.Len(t, imported, 1) {
				assert.Equal(t, role, imported[0].Get("keytype"))
			}
		})
	}
}

func TestCryptokeyReadUsesSeparateServerRoles(t *testing.T) {
	for _, roles := range []struct{ previous, reported string }{
		{"zsk", "ksk"}, {"csk", "zsk"},
	} {
		t.Run(roles.previous+"_to_"+roles.reported, func(t *testing.T) {
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":1,"keytype":%q,"active":true,"published":true}`, roles.reported)), nil
			})
			res := resourcePDNSZoneCryptokey()
			data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
				"zone": "example.com.", "keytype": roles.previous,
			})
			data.SetId("example.com.:1")
			diags := res.ReadContext(context.Background(), data, &ProviderClients{PDNS: client})
			if assert.False(t, diags.HasError(), "%v", diags) {
				assert.Equal(t, roles.reported, data.Get("keytype"), "changes between KSK/CSK and ZSK are not normalizations")
			}
		})
	}
}

func TestAccPDNSZoneCryptokeyRoleChanges(t *testing.T) {
	const resourceName = "powerdns_zone_cryptokey.role-change"
	var previousDNSKey string
	var steps []resource.TestStep
	for _, role := range []string{"csk", "ksk", "zsk", "csk"} {
		config := fmt.Sprintf(`
resource "powerdns_zone" "cryptokey-role-change" {
  name = "cryptokey-role-change.sysa.abc."
  kind = "Native"
}

resource "powerdns_zone_cryptokey" "role-change" {
  zone      = powerdns_zone.cryptokey-role-change.name
  keytype   = %q
  algorithm = "ecdsa256"
}`, role)
		steps = append(steps,
			resource.TestStep{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneCryptokeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "keytype", role),
					func(s *terraform.State) error {
						dnskey := s.RootModule().Resources[resourceName].Primary.Attributes["dnskey"]
						if dnskey == "" || dnskey == previousDNSKey {
							return fmt.Errorf("changing the key role to %s did not generate new key material", role)
						}
						previousDNSKey = dnskey
						return nil
					},
				),
			},
			// Server normalization must not produce a replacement on the next plan.
			resource.TestStep{Config: config, PlanOnly: true},
		)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSZoneCryptokeyDestroy,
		Steps:             steps,
	})
}

func TestAccPDNSZoneCryptokeyGenerated(t *testing.T) {
	resourceName := "powerdns_zone_cryptokey.test-generated"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneCryptokeyConfigGenerated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneCryptokeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "zone", "cryptokey-generated.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "active", "true"),
					resource.TestCheckResourceAttr(resourceName, "published", "true"),
					// PowerDNS generates the key material and reports it back.
					resource.TestCheckResourceAttrSet(resourceName, "key_id"),
					resource.TestCheckResourceAttrSet(resourceName, "dnskey"),
					resource.TestCheckResourceAttrSet(resourceName, "algorithm"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// The private key is never returned on a plain read.
				ImportStateVerifyIgnore: []string{"content"},
			},
		},
	})
}

func TestAccPDNSZoneCryptokeyActiveToggle(t *testing.T) {
	resourceName := "powerdns_zone_cryptokey.test-toggle"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneCryptokeyConfigToggle,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneCryptokeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "active", "true"),
					resource.TestCheckResourceAttr(resourceName, "published", "true"),
				),
			},
			{
				// Both flags are sent together, since PowerDNS rejects an update
				// that omits "active".
				Config: testPDNSZoneCryptokeyConfigToggled,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneCryptokeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "active", "false"),
					resource.TestCheckResourceAttr(resourceName, "published", "false"),
				),
			},
		},
	})
}

func TestAccPDNSZoneCryptokeyRSAWithBits(t *testing.T) {
	resourceName := "powerdns_zone_cryptokey.test-rsa"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				// RSA requires an explicit key size.
				Config: testPDNSZoneCryptokeyConfigRSA,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneCryptokeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "bits", "2048"),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "RSASHA256"),
				),
			},
			{
				// The short mnemonic in the config must not read as a change.
				Config:   testPDNSZoneCryptokeyConfigRSA,
				PlanOnly: true,
			},
		},
	})
}

func TestAccPDNSZoneCryptokeyInvalidKeytype(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneCryptokeyConfigInvalidKeytype,
				ExpectError: regexp.MustCompile(`expected keytype to be one of`),
			},
		},
	})
}

func TestParseCryptokeyID(t *testing.T) {
	zone, id, err := parseCryptokeyID("example.com.:3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if zone != "example.com." {
		t.Errorf("zone = %q, want %q", zone, "example.com.")
	}
	if id != 3 {
		t.Errorf("id = %d, want 3", id)
	}

	for _, badID := range []string{"example.com.", "example.com.:", ":3", "example.com.:abc", ""} {
		if _, _, err := parseCryptokeyID(badID); err == nil {
			t.Errorf("parseCryptokeyID(%q) succeeded, want error", badID)
		}
	}
}

func TestNormalizeCryptokeyAlgorithm(t *testing.T) {
	cases := map[string]string{
		"ecdsa256":        "ECDSAP256SHA256",
		"ECDSA256":        "ECDSAP256SHA256",
		"ECDSAP256SHA256": "ECDSAP256SHA256",
		"ecdsa384":        "ECDSAP384SHA384",
		"rsasha256":       "RSASHA256",
		"ed25519":         "ED25519",
		// Unknown values are simply upper-cased.
		"something-else": "SOMETHING-ELSE",
	}

	for input, want := range cases {
		if got := NormalizeCryptokeyAlgorithm(input); got != want {
			t.Errorf("NormalizeCryptokeyAlgorithm(%q) = %q, want %q", input, got, want)
		}
	}
}

func testAccCheckPDNSZoneCryptokeyDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_zone_cryptokey" {
			continue
		}

		zone, id, err := parseCryptokeyID(rs.Primary.ID)
		if err != nil {
			return err
		}

		client := testAccProvider.Meta().(*ProviderClients)
		_, err = client.PDNS.GetCryptokey(context.Background(), zone, id)
		if err == nil {
			return fmt.Errorf("cryptokey %s still exists", rs.Primary.ID)
		}
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("error checking cryptokey %s is gone: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckPDNSZoneCryptokeyExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no cryptokey ID is set")
		}

		zone, id, err := parseCryptokeyID(rs.Primary.ID)
		if err != nil {
			return err
		}

		client := testAccProvider.Meta().(*ProviderClients)
		if _, err := client.PDNS.GetCryptokey(context.Background(), zone, id); err != nil {
			return fmt.Errorf("error fetching cryptokey %s: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

const testPDNSZoneCryptokeyConfigGenerated = `
resource "powerdns_zone" "cryptokey-generated" {
	name = "cryptokey-generated.sysa.abc."
	kind = "Native"
}

resource "powerdns_zone_cryptokey" "test-generated" {
	zone      = powerdns_zone.cryptokey-generated.name
	keytype   = "csk"
	algorithm = "ecdsa256"
}`

const testPDNSZoneCryptokeyConfigToggle = `
resource "powerdns_zone" "cryptokey-toggle" {
	name = "cryptokey-toggle.sysa.abc."
	kind = "Native"
}

resource "powerdns_zone_cryptokey" "test-toggle" {
	zone      = powerdns_zone.cryptokey-toggle.name
	keytype   = "csk"
	algorithm = "ecdsa256"
	active    = true
	published = true
}`

const testPDNSZoneCryptokeyConfigToggled = `
resource "powerdns_zone" "cryptokey-toggle" {
	name = "cryptokey-toggle.sysa.abc."
	kind = "Native"
}

resource "powerdns_zone_cryptokey" "test-toggle" {
	zone      = powerdns_zone.cryptokey-toggle.name
	keytype   = "csk"
	algorithm = "ecdsa256"
	active    = false
	published = false
}`

const testPDNSZoneCryptokeyConfigInvalidKeytype = `
resource "powerdns_zone_cryptokey" "test-invalid" {
	zone      = "cryptokey-invalid.sysa.abc."
	keytype   = "not-a-keytype"
	algorithm = "ecdsa256"
}`

const testPDNSZoneCryptokeyConfigRSA = `
resource "powerdns_zone" "cryptokey-rsa" {
	name = "cryptokey-rsa.sysa.abc."
	kind = "Native"
}

resource "powerdns_zone_cryptokey" "test-rsa" {
	zone      = powerdns_zone.cryptokey-rsa.name
	keytype   = "csk"
	algorithm = "rsasha256"
	bits      = 2048
}`
