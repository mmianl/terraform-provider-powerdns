package powerdns

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccPDNSRecordSOA_Basic(t *testing.T) {
	resourceName := "powerdns_record_soa.test-soa"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordSOADestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordSOAConfigBasic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					testAccCheckPDNSRecordSOAIDFormat(resourceName, "test-soa2-sysa.xyz.:::SOA"),
					resource.TestCheckResourceAttr(resourceName, "zone", "test-soa2-sysa.xyz."),
					resource.TestCheckResourceAttr(resourceName, "name", "test-soa2-sysa.xyz."),
					resource.TestCheckResourceAttr(resourceName, "ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "mname", "ns1.sysa.xyz."),
					resource.TestCheckResourceAttr(resourceName, "rname", "hostmaster.sysa.xyz."),
					resource.TestCheckResourceAttr(resourceName, "refresh", "7200"),
					resource.TestCheckResourceAttr(resourceName, "retry", "600"),
					resource.TestCheckResourceAttr(resourceName, "expire", "1209600"),
					resource.TestCheckResourceAttr(resourceName, "minimum", "6400"),
				),
			},
		},
	})
}

func TestAccPDNSRecordSOA_Update(t *testing.T) {
	resourceName := "powerdns_record_soa.test-soa-upd"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordSOADestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordSOAConfigUpdate1,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "refresh", "7200"),
				),
			},
			{
				Config: testPDNSRecordSOAConfigUpdate2,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "ttl", "7200"),
					resource.TestCheckResourceAttr(resourceName, "refresh", "14400"),
				),
			},
		},
	})
}

func TestAccPDNSRecordSOA_Import(t *testing.T) {
	resourceName := "powerdns_record_soa.test-soa-imp"
	resourceID := `{"zone":"test-soa-imp-sysa.xyz.","id":"test-soa-imp-sysa.xyz.:::SOA"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordSOADestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordSOAConfigImport,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					testAccCheckPDNSRecordSOAIDFormat(resourceName, "test-soa-imp-sysa.xyz.:::SOA"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 state, got %d", len(states))
					}
					s := states[0]
					if s.ID != "test-soa-imp-sysa.xyz.:::SOA" {
						return fmt.Errorf("imported ID: got %q, want %q", s.ID, "test-soa-imp-sysa.xyz.:::SOA")
					}
					checks := map[string]string{
						"zone":  "test-soa-imp-sysa.xyz.",
						"name":  "test-soa-imp-sysa.xyz.",
						"mname": "ns1.sysa.xyz.",
						"rname": "hostmaster.sysa.xyz.",
					}
					for k, want := range checks {
						if got := s.Attributes[k]; got != want {
							return fmt.Errorf("imported %s: got %q, want %q", k, got, want)
						}
					}
					return nil
				},
			},
		},
	})
}

func TestAccPDNSRecordSOA_BlockedInRecord(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSRecordSOABlockedConfig,
				ExpectError: regexp.MustCompile("use the powerdns_record_soa resource instead"),
			},
		},
	})
}

func testAccCheckPDNSRecordSOADestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_record_soa" {
			continue
		}

		client := testAccProvider.Meta().(*ProviderClients)
		exists, err := client.PDNS.RecordExistsByID(context.Background(), rs.Primary.Attributes["zone"], rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error checking if SOA record still exists: %#v", rs.Primary.ID)
		}
		if exists {
			return fmt.Errorf("SOA record still exists: %#v", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCheckPDNSRecordSOAIDFormat(resourceName, expectedID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}
		if rs.Primary.ID != expectedID {
			return fmt.Errorf("SOA record ID: got %q, want %q", rs.Primary.ID, expectedID)
		}
		return nil
	}
}

func testAccCheckPDNSRecordSOAExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No SOA Record ID is set")
		}

		client := testAccProvider.Meta().(*ProviderClients)
		foundRecords, err := client.PDNS.ListRecordsByID(context.Background(), rs.Primary.Attributes["zone"], rs.Primary.ID)
		if err != nil {
			return err
		}
		if len(foundRecords) == 0 {
			return fmt.Errorf("SOA record does not exist")
		}
		return nil
	}
}

const testPDNSRecordSOAConfigBasic = `
resource "powerdns_zone" "test-soa2" {
	name         = "test-soa2-sysa.xyz."
	kind         = "MASTER"
}

resource "powerdns_record_soa" "test-soa" {
	zone    = powerdns_zone.test-soa2.name
	name    = "test-soa2-sysa.xyz."
	ttl     = 3600
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	serial  = 0
	refresh = 7200
	retry   = 600
	expire  = 1209600
	minimum = 6400
}`

const testPDNSRecordSOAConfigUpdate1 = `
resource "powerdns_zone" "test-soa-upd" {
	name         = "test-soa-upd-sysa.xyz."
	kind         = "MASTER"
}

resource "powerdns_record_soa" "test-soa-upd" {
	zone    = powerdns_zone.test-soa-upd.name
	name    = "test-soa-upd-sysa.xyz."
	ttl     = 3600
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	serial  = 0
	refresh = 7200
	retry   = 600
	expire  = 1209600
	minimum = 6400
}`

const testPDNSRecordSOAConfigUpdate2 = `
resource "powerdns_zone" "test-soa-upd" {
	name         = "test-soa-upd-sysa.xyz."
	kind         = "MASTER"
}

resource "powerdns_record_soa" "test-soa-upd" {
	zone    = powerdns_zone.test-soa-upd.name
	name    = "test-soa-upd-sysa.xyz."
	ttl     = 7200
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	serial  = 0
	refresh = 14400
	retry   = 600
	expire  = 1209600
	minimum = 6400
}`

const testPDNSRecordSOAConfigImport = `
resource "powerdns_zone" "test-soa-imp" {
	name         = "test-soa-imp-sysa.xyz."
	kind         = "MASTER"
}

resource "powerdns_record_soa" "test-soa-imp" {
	zone    = powerdns_zone.test-soa-imp.name
	name    = "test-soa-imp-sysa.xyz."
	ttl     = 3600
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	serial  = 0
	refresh = 7200
	retry   = 600
	expire  = 1209600
	minimum = 6400
}`

const testPDNSRecordSOABlockedConfig = `
resource "powerdns_record" "test-soa-blocked" {
	zone    = "test-soa-blocked-sysa.xyz."
	name    = "test-soa-blocked-sysa.xyz."
	type    = "SOA"
	ttl     = 3600
	records = [ "ns1.sysa.xyz. hostmaster.sysa.xyz. 2019090301 10800 3600 604800 3600" ]
}`
