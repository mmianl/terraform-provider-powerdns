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

func TestCryptokeyRoleChangesRequireReplacement(t *testing.T) {
	for _, roles := range []struct{ requested, reported string }{
		{"csk", "csk"}, {"ksk", "csk"}, {"zsk", "csk"},
		{"csk", "ksk"}, {"ksk", "ksk"}, {"zsk", "zsk"},
	} {
		oldRole := roles.requested
		for _, newRole := range []string{"csk", "ksk", "zsk"} {
			t.Run(oldRole+"_reported_as_"+roles.reported+"_to_"+newRole, func(t *testing.T) {
				client := newTestClient(func(r *http.Request) (*http.Response, error) {
					assert.Equal(t, http.MethodGet, r.Method)
					return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":1,"keytype":%q,"algorithm":"ECDSAP256SHA256","bits":256,"active":true,"published":true}`, roles.reported)), nil
				})
				h := newResourceHarness(t, newZoneCryptokeyResource(), &ProviderClients{PDNS: client})
				state := h.object(map[string]tftypes.Value{
					"id": tfString("example.com.:1"), "zone": tfString("example.com."),
					"keytype": tfString(oldRole), "algorithm": tfString("ecdsa256"),
				})
				// Refresh twice to ensure the requested role survives in saved state.
				for i := 0; i < 2; i++ {
					refreshed, diags := h.read(state)
					if !assert.False(t, diags.HasError(), "%v", diags) {
						return
					}
					state = refreshed
					assert.Equal(t, oldRole, tfAttrString(t, state, "keytype"))
				}
				// The role is replace-only: an unchanged role is the same value, any
				// other role is a different one and therefore forces a new key.
				equal, diags := newSemanticString(caseInsensitiveStringType, newRole).StringSemanticEquals(
					context.Background(), newSemanticString(caseInsensitiveStringType, tfAttrString(t, state, "keytype")))
				assert.False(t, diags.HasError())
				assert.Equal(t, oldRole == newRole, equal)
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
			h := newResourceHarness(t, newZoneCryptokeyResource(), &ProviderClients{PDNS: client})
			imported, diags := h.importState("example.com.:1")
			if assert.False(t, diags.HasError(), "%v", diags) {
				assert.Equal(t, role, tfAttrString(t, imported, "keytype"))
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
			h := newResourceHarness(t, newZoneCryptokeyResource(), &ProviderClients{PDNS: client})
			state := h.object(map[string]tftypes.Value{
				"id": tfString("example.com.:1"), "zone": tfString("example.com."), "keytype": tfString(roles.previous),
			})
			refreshed, diags := h.read(state)
			if assert.False(t, diags.HasError(), "%v", diags) {
				assert.Equal(t, roles.reported, tfAttrString(t, refreshed, "keytype"), "changes between KSK/CSK and ZSK are not normalizations")
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSZoneCryptokeyDestroy,
		Steps:                    steps,
	})
}

func TestAccPDNSZoneCryptokeyGenerated(t *testing.T) {
	resourceName := "powerdns_zone_cryptokey.test-generated"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSZoneCryptokeyDestroy,
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
				// The private key is never returned on a plain read, and the algorithm
				// keeps the configured spelling in state.
				ImportStateVerifyIgnore: []string{"content", "algorithm"},
			},
		},
	})
}

func TestAccPDNSZoneCryptokeyActiveToggle(t *testing.T) {
	resourceName := "powerdns_zone_cryptokey.test-toggle"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSZoneCryptokeyDestroy,
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				// RSA requires an explicit key size.
				Config: testPDNSZoneCryptokeyConfigRSA,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneCryptokeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "bits", "2048"),
					// State keeps the spelling from the configuration.
					resource.TestCheckResourceAttr(resourceName, "algorithm", "rsasha256"),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSZoneCryptokeyDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneCryptokeyConfigInvalidKeytype,
				ExpectError: regexp.MustCompile(`keytype\s+value\s+must\s+be\s+one\s+of`),
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
