package powerdns

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// volatileZoneSerials are maintained by PowerDNS rather than by configuration.
// A zone that notifies its slaves between the apply and the import step comes
// back with a different notified_serial, so verifying them on import is racy.
var volatileZoneSerials = []string{"serial", "notified_serial", "edited_serial", "last_check"}

func TestAccPDNSZoneNative(t *testing.T) {
	resourceName := "powerdns_zone.test-native"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigNative,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Native"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneNativeMixedCaps(t *testing.T) {
	resourceName := "powerdns_zone.test-native"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				// using mixed caps config to create resource with test-native name
				Config: testPDNSZoneConfigNativeMixedCaps,
			},
			{
				// using test-native config with Native to confirm plan doesn't generate diff
				ResourceName:       resourceName,
				Config:             testPDNSZoneConfigNative,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
			},
		},
	})
}

func TestAccPDNSZoneNativeSmallCaps(t *testing.T) {
	resourceName := "powerdns_zone.test-native"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				// using small caps config to create resource with test-native name
				Config: testPDNSZoneConfigNativeSmallCaps,
			},
			{
				// using test-native config with Native to confirm plan doesn't generate diff
				ResourceName:       resourceName,
				Config:             testPDNSZoneConfigNative,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
			},
		},
	})
}

func TestAccPDNSZoneMaster(t *testing.T) {
	resourceName := "powerdns_zone.test-master"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMaster,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneMasterSOAAPIEDIT(t *testing.T) {
	resourceName := "powerdns_zone.test-master-soa-edit-api"
	resourceSOAEDITAPI := `DEFAULT`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMasterSOAEDITAPI,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master-soa-edit-api.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "soa_edit_api", resourceSOAEDITAPI),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneMasterSOAAPIEDITEmpty(t *testing.T) {
	resourceName := "powerdns_zone.test-master-soa-edit-api-empty"
	resourceSOAEDITAPI := `""`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMasterSOAEDITAPIEmpty,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master-soa-edit-api-empty.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "soa_edit_api", resourceSOAEDITAPI),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneMasterSOAAPIEDITUndefined(t *testing.T) {
	resourceName := "powerdns_zone.test-master-soa-edit-api-undefined"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMasterSOAEDITAPIUndefined,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master-soa-edit-api-undefined.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneAccount(t *testing.T) {
	resourceName := "powerdns_zone.test-account"
	resourceAccount := `test`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigAccount,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "account.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "account", resourceAccount),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneCatalog(t *testing.T) {
	resourceName := "powerdns_zone.test-catalog"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigCatalog,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "catalog.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "catalog", "catalog-a.example."),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneDNSSec(t *testing.T) {
	resourceName := "powerdns_zone.test-dnssec"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigDNSSec,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "dnssec.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "dnssec", "true"),
					resource.TestCheckResourceAttr(resourceName, "soa_edit", "INCEPTION-INCREMENT"),
					resource.TestCheckResourceAttr(resourceName, "api_rectify", "true"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
			{
				// DNSSEC is togglable in place, so this must update rather than
				// force a replacement.
				Config: testPDNSZoneConfigDNSSecDisabled,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "dnssec", "false"),
				),
			},
		},
	})
}

func TestAccPDNSZoneComputedSerials(t *testing.T) {
	resourceName := "powerdns_zone.test-serials"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigComputedSerials,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "serial"),
					resource.TestCheckResourceAttrSet(resourceName, "edited_serial"),
					resource.TestCheckResourceAttr(resourceName, "notified_serial", "0"),
					resource.TestCheckResourceAttr(resourceName, "last_check", "0"),
				),
			},
		},
	})
}

func TestAccPDNSZoneTSIGKeyRefs(t *testing.T) {
	resourceName := "powerdns_zone.test-tsig-refs"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigTSIGKeyRefs,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "master_tsig_key_ids.#", "1"),
					// The reference is stored in the ID form PowerDNS reports back.
					resource.TestCheckTypeSetElemAttr(resourceName, "master_tsig_key_ids.*", "tf-zone-ref."),
					resource.TestCheckResourceAttr(resourceName, "slave_tsig_key_ids.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "slave_tsig_key_ids.*", "tf-zone-ref."),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneTSIGKeyRefsBareName(t *testing.T) {
	resourceName := "powerdns_zone.test-tsig-bare"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigTSIGKeyBareName,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					// A bare name is normalized to the dotted ID form.
					resource.TestCheckTypeSetElemAttr(resourceName, "master_tsig_key_ids.*", "tf-zone-bare."),
				),
			},
			{
				// PowerDNS rewrites a bare name to its ID, so without normalization
				// this configuration would never converge.
				Config:   testPDNSZoneConfigTSIGKeyBareName,
				PlanOnly: true,
			},
		},
	})
}

func TestAccPDNSZoneAccountEmpty(t *testing.T) {
	resourceName := "powerdns_zone.test-account-empty"
	resourceAccount := ``

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigAccountEmpty,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "account-empty.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "account", resourceAccount),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneAccountUndefined(t *testing.T) {
	resourceName := "powerdns_zone.test-account-undefined"
	resourceAccount := `admin`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigAccountUndefined,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "account-undefined.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "account", resourceAccount),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneSlave(t *testing.T) {
	resourceName := "powerdns_zone.test-slave"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlave,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "slave.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Slave"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithMasters(t *testing.T) {
	resourceName := "powerdns_zone.test-slave-with-masters"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		//CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlaveWithMasters,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "slave-with-masters.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Slave"),
					resource.TestCheckTypeSetElemAttr(resourceName, "masters.*", "1.1.1.1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "masters.*", "2.2.2.2"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithMastersWithPort(t *testing.T) {
	resourceName := "powerdns_zone.test-slave-with-masters-with-port"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		//CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlaveWithMastersWithPort,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "slave-with-masters-with-port.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Slave"),
					resource.TestCheckTypeSetElemAttr(resourceName, "masters.*", "2.2.2.2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "masters.*", "1.1.1.1:1111"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// PowerDNS maintains these itself and can bump them between the
				// apply and the import, so they are not stable enough to verify.
				ImportStateVerifyIgnore: volatileZoneSerials,
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithIPv6Masters(t *testing.T) {
	resourceName := "powerdns_zone.test-slave-with-ipv6-masters"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlaveWithIPv6Masters,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "masters.#", "2"),
					// Stored in the canonical compressed form, which is also what
					// PowerDNS returns — so a re-plan is empty.
					resource.TestCheckTypeSetElemAttr(resourceName, "masters.*",
						"fd92:81e1:e314:ea7b:0:1234:5678:60ab"),
					resource.TestCheckTypeSetElemAttr(resourceName, "masters.*", "192.168.123.45"),
				),
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithMastersWithInvalidPort(t *testing.T) {

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigSlaveWithMastersWithInvalidPort,
				ExpectError: regexp.MustCompile("invalid port value in masters attribute"),
			},
		},
	})
}
func TestAccPDNSZoneSlaveWithInvalidMasters(t *testing.T) {

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigSlaveWithInvalidMasters,
				ExpectError: regexp.MustCompile("values in masters list attribute must be valid IPs"),
			},
		},
	})
}

func TestAccPDNSZoneMasterWithMasters(t *testing.T) {

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigMasterWithMasters,
				ExpectError: regexp.MustCompile("masters attribute is supported only for Slave kind"),
			},
		},
	})
}

func TestAccPDNSZoneInvalidDomain(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigInvalidDomain,
				ExpectError: regexp.MustCompile("fully qualified domain name ending with a trailing dot"),
			},
		},
	})
}

func TestAccPDNSZoneInvalidVariant(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigInvalidVariant,
				ExpectError: regexp.MustCompile("variant name may contain only lowercase letters"),
			},
		},
	})
}

func TestAccPDNSZoneInvalidVariantTrailingDot(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigInvalidVariantTrailingDot,
				ExpectError: regexp.MustCompile("variant name must not end with a dot"),
			},
		},
	})
}

func TestAccPDNSZoneInvalidVariantMultipleSeparators(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigInvalidVariantMultipleSeparators,
				ExpectError: regexp.MustCompile("only one variant separator"),
			},
		},
	})
}

func testAccCheckPDNSZoneDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_zone" {
			continue
		}

		client := testAccProvider.Meta().(*ProviderClients).PDNS
		exists, err := client.ZoneExists(context.Background(), rs.Primary.Attributes["name"])
		if err != nil {
			return fmt.Errorf("Error checking if zone still exists: %#v", rs.Primary.ID)
		}
		if exists {
			return fmt.Errorf("Zone still exists: %#v", rs.Primary.ID)
		}

	}
	return nil
}

func testAccCheckPDNSZoneExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		client := testAccProvider.Meta().(*ProviderClients).PDNS
		exists, err := client.ZoneExists(context.Background(), rs.Primary.Attributes["name"])
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("Zone does not exist: %#v", rs.Primary.ID)
		}
		return nil
	}
}

const testPDNSZoneConfigNative = `
resource "powerdns_zone" "test-native" {
	name = "sysa.abc."
	kind = "Native"
}`

const testPDNSZoneConfigNativeMixedCaps = `
resource "powerdns_zone" "test-native" {
	name = "sysa.abc."
	kind = "NaTIve"
}`

const testPDNSZoneConfigNativeSmallCaps = `
resource "powerdns_zone" "test-native" {
	name = "sysa.abc."
	kind = "native"
}`

const testPDNSZoneConfigMaster = `
resource "powerdns_zone" "test-master" {
	name = "master.sysa.abc."
	kind = "Master"
}`

const testPDNSZoneConfigInvalidDomain = `
resource "powerdns_zone" "test-invalid-domain" {
	name = "invalid.example.com"
	kind = "Native"
}`

const testPDNSZoneConfigInvalidVariant = `
resource "powerdns_zone" "test-invalid-variant" {
	name = "invalid.example.com..BadVariant"
	kind = "Native"
}`

const testPDNSZoneConfigInvalidVariantTrailingDot = `
resource "powerdns_zone" "test-invalid-variant-trailing-dot" {
	name = "invalid.example.com..variant."
	kind = "Native"
}`

const testPDNSZoneConfigInvalidVariantMultipleSeparators = `
resource "powerdns_zone" "test-invalid-variant-multiple-separators" {
	name = "invalid.example.com..variant..blue"
	kind = "Native"
}`

const testPDNSZoneConfigMasterSOAEDITAPI = `
resource "powerdns_zone" "test-master-soa-edit-api" {
	name = "master-soa-edit-api.sysa.abc."
	kind = "Master"
	soa_edit_api = "DEFAULT"
}`

const testPDNSZoneConfigMasterSOAEDITAPIEmpty = `
resource "powerdns_zone" "test-master-soa-edit-api-empty" {
	name = "master-soa-edit-api-empty.sysa.abc."
	kind = "Master"
	soa_edit_api = "\"\""
}`

const testPDNSZoneConfigMasterSOAEDITAPIUndefined = `
resource "powerdns_zone" "test-master-soa-edit-api-undefined" {
	name = "master-soa-edit-api-undefined.sysa.abc."
	kind = "Master"
}`

const testPDNSZoneConfigAccount = `
resource "powerdns_zone" "test-account" {
	name = "account.sysa.abc."
	kind = "Master"
	account = "test"
}`

const testPDNSZoneConfigAccountEmpty = `
resource "powerdns_zone" "test-account-empty" {
	name = "account-empty.sysa.abc."
	kind = "Master"
	account = ""
}`

const testPDNSZoneConfigAccountUndefined = `
resource "powerdns_zone" "test-account-undefined" {
	name = "account-undefined.sysa.abc."
	kind = "Master"
	soa_edit_api = "DEFAULT"
}`

const testPDNSZoneConfigSlave = `
resource "powerdns_zone" "test-slave" {
	name = "slave.sysa.abc."
	kind = "Slave"
}`

const testPDNSZoneConfigCatalog = `
resource "powerdns_zone" "test-catalog" {
	name    = "catalog.sysa.abc."
	kind    = "Master"
	catalog = "catalog-a.example."
}`

const testPDNSZoneConfigSlaveWithMasters = `
resource "powerdns_zone" "test-slave-with-masters" {
	name = "slave-with-masters.sysa.abc."
	kind = "Slave"
	masters = ["1.1.1.1", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithMastersWithPort = `
resource "powerdns_zone" "test-slave-with-masters-with-port" {
	name = "slave-with-masters-with-port.sysa.abc."
	kind = "Slave"
	masters = ["1.1.1.1:1111", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithMastersWithInvalidPort = `
resource "powerdns_zone" "test-slave-with-masters-with-invalid-port" {
	name = "slave-with-masters-with-invalid-port.sysa.abc."
	kind = "Slave"
	masters = ["1.1.1.1:111111", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithInvalidMasters = `
resource "powerdns_zone" "test-slave-with-invalid-masters" {
	name = "slave-with-invalid-masters.sysa.abc."
	kind = "Slave"
	masters = ["example.com", "2.2.2.2"]
}`

const testPDNSZoneConfigMasterWithMasters = `
resource "powerdns_zone" "test-master-with-masters" {
	name = "master-with-masters.sysa.abc."
	kind = "Master"
	masters = ["1.1.1.1", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithIPv6Masters = `
resource "powerdns_zone" "test-slave-with-ipv6-masters" {
	name = "slave-with-ipv6-masters.sysa.abc."
	kind = "Slave"
	soa_edit_api = "DEFAULT"
	masters = [
		"fd92:81e1:e314:ea7b:0000:1234:5678:60ab",
		"192.168.123.45",
	]
}`

const testPDNSZoneConfigDNSSec = `
resource "powerdns_zone" "test-dnssec" {
	name        = "dnssec.sysa.abc."
	kind        = "Native"
	dnssec      = true
	soa_edit    = "INCEPTION-INCREMENT"
	api_rectify = true
}`

const testPDNSZoneConfigDNSSecDisabled = `
resource "powerdns_zone" "test-dnssec" {
	name        = "dnssec.sysa.abc."
	kind        = "Native"
	dnssec      = false
	soa_edit    = "INCEPTION-INCREMENT"
	api_rectify = true
}`

const testPDNSZoneConfigComputedSerials = `
resource "powerdns_zone" "test-serials" {
	name = "serials.sysa.abc."
	kind = "Native"
}`

const testPDNSZoneConfigTSIGKeyRefs = `
resource "powerdns_tsigkey" "zone-ref" {
	name      = "tf-zone-ref"
	algorithm = "hmac-sha256"
}

resource "powerdns_zone" "test-tsig-refs" {
	name                = "tsig-refs.sysa.abc."
	kind                = "Master"
	master_tsig_key_ids = [powerdns_tsigkey.zone-ref.id]
	slave_tsig_key_ids  = [powerdns_tsigkey.zone-ref.id]
}`

const testPDNSZoneConfigTSIGKeyBareName = `
resource "powerdns_tsigkey" "zone-bare" {
	name      = "tf-zone-bare"
	algorithm = "hmac-sha256"
}

resource "powerdns_zone" "test-tsig-bare" {
	name                = "tsig-bare.sysa.abc."
	kind                = "Master"
	master_tsig_key_ids = [powerdns_tsigkey.zone-bare.name]
}`
