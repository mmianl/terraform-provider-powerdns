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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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

func TestAccPDNSTSIGKeyWriteOnly(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-wo"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigWriteOnly(testTSIGSecret, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					// The secret must never reach state.
					resource.TestCheckNoResourceAttr(resourceName, "key_wo"),
					resource.TestCheckNoResourceAttr(resourceName, "key"),
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
				),
			},
			{
				// A new value without a new version is not noticed.
				Config:   testPDNSTSIGKeyConfigWriteOnly(testTSIGSecretRotated, 1),
				PlanOnly: true,
			},
			{
				Config: testPDNSTSIGKeyConfigWriteOnly(testTSIGSecretRotated, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecretRotated),
					resource.TestCheckNoResourceAttr(resourceName, "key"),
				),
			},
		},
	})
}

func TestAccPDNSTSIGKeyExplicitSecret(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-explicit"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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

// testAccCheckPDNSTSIGKeySecret asks the server, since state must not hold a
// write-only secret.
func testAccCheckPDNSTSIGKeySecret(n string, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		client := testAccProvider.Meta().(*ProviderClients)
		got, err := client.PDNS.GetTSIGKey(context.Background(), rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error fetching TSIG key %s: %w", rs.Primary.ID, err)
		}
		if got.Key != want {
			return fmt.Errorf("server holds a different secret than the configured write-only value")
		}
		return nil
	}
}

func testPDNSTSIGKeyConfigWriteOnly(secret string, version int) string {
	return fmt.Sprintf(`
resource "powerdns_tsigkey" "test-wo" {
	name           = "tf-write-only"
	algorithm      = "hmac-sha256"
	key_wo         = %q
	key_wo_version = %d
}`, secret, version)
}
