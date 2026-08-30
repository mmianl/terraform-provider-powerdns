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

func TestAccPDNSTSIGKeyGenerated(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-generated"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigGenerated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "tf-generated"),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "hmac-sha256"),
					// The server generates the secret when none is supplied.
					resource.TestCheckResourceAttrSet(resourceName, "key"),
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

func TestAccPDNSTSIGKeyExplicitSecret(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-explicit"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigExplicit,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "tf-explicit"),
					resource.TestCheckResourceAttr(resourceName, "key", testTSIGSecret),
				),
			},
			{
				// Rotating the secret is an in-place update.
				Config: testPDNSTSIGKeyConfigExplicitRotated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "key", testTSIGSecretRotated),
				),
			},
		},
	})
}

func TestAccPDNSTSIGKeyAlgorithmForcesNew(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-algorithm"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigAlgorithm,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "hmac-sha256"),
				),
			},
			{
				// PowerDNS would otherwise keep the old algorithm alongside the
				// new one under the same name, so this must replace the key.
				Config: testPDNSTSIGKeyConfigAlgorithmChanged,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "hmac-sha512"),
				),
			},
		},
	})
}

func TestAccPDNSTSIGKeyInvalidAlgorithm(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSTSIGKeyConfigInvalidAlgorithm,
				ExpectError: regexp.MustCompile(`expected algorithm to be one of`),
			},
		},
	})
}

func testAccCheckPDNSTSIGKeyDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_tsigkey" {
			continue
		}

		client := testAccProvider.Meta().(*ProviderClients)
		_, err := client.PDNS.GetTSIGKey(context.Background(), rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("TSIG key %s still exists", rs.Primary.ID)
		}
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("error checking TSIG key %s is gone: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckPDNSTSIGKeyExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no TSIG key ID is set")
		}

		client := testAccProvider.Meta().(*ProviderClients)
		if _, err := client.PDNS.GetTSIGKey(context.Background(), rs.Primary.ID); err != nil {
			return fmt.Errorf("error fetching TSIG key %s: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

const testTSIGSecret = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3OD0="
const testTSIGSecretRotated = "MTIzNDU2Nzg5MGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHk="

const testPDNSTSIGKeyConfigGenerated = `
resource "powerdns_tsigkey" "test-generated" {
	name      = "tf-generated"
	algorithm = "hmac-sha256"
}`

const testPDNSTSIGKeyConfigExplicit = `
resource "powerdns_tsigkey" "test-explicit" {
	name      = "tf-explicit"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecret + `"
}`

const testPDNSTSIGKeyConfigExplicitRotated = `
resource "powerdns_tsigkey" "test-explicit" {
	name      = "tf-explicit"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecretRotated + `"
}`

const testPDNSTSIGKeyConfigAlgorithm = `
resource "powerdns_tsigkey" "test-algorithm" {
	name      = "tf-algorithm"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecret + `"
}`

const testPDNSTSIGKeyConfigAlgorithmChanged = `
resource "powerdns_tsigkey" "test-algorithm" {
	name      = "tf-algorithm"
	algorithm = "hmac-sha512"
	key       = "` + testTSIGSecret + `"
}`

const testPDNSTSIGKeyConfigInvalidAlgorithm = `
resource "powerdns_tsigkey" "test-invalid" {
	name      = "tf-invalid"
	algorithm = "hmac-nonsense"
}`

func TestNormalizeTSIGKeyID(t *testing.T) {
	cases := map[string]string{
		"mykey":  "mykey.",
		"mykey.": "mykey.",
		// An empty reference stays empty rather than becoming a bare dot.
		"": "",
	}

	for input, want := range cases {
		if got := NormalizeTSIGKeyID(input); got != want {
			t.Errorf("NormalizeTSIGKeyID(%q) = %q, want %q", input, got, want)
		}
	}
}
