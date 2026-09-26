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

func TestAccPDNSZoneCryptokeyGenerated(t *testing.T) {
	resourceName := "powerdns_zone_cryptokey.test-generated"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneCryptokeyDestroy,
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
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneCryptokeyDestroy,
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
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneCryptokeyDestroy,
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
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneCryptokeyDestroy,
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
