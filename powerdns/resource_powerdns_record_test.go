package powerdns

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccPDNSRecord_Empty(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSRecordConfigRecordEmpty,
				ExpectError: regexp.MustCompile("'records' must not be empty"),
			},
		},
	})
}

func TestAccPDNSRecord_A(t *testing.T) {
	resourceName := "powerdns_record.test-a"
	resourceID := `{"zone":"a.sysa.xyz.","id":"testpdnsrecordconfiga.a.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigA,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_WithPtr(t *testing.T) {
	resourceName := "powerdns_record.test-a-ptr"
	resourceID := `{"zone":"ptr.sysa.xyz.","id":"testpdnsrecordconfigawithptr.ptr.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigAWithPtr,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:            resourceName,
				ImportStateId:           resourceID,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"set_ptr"},
			},
		},
	})
}

func TestAccPDNSRecord_WithCount(t *testing.T) {
	resourceID0 := `{"zone":"count.sysa.xyz.","id":"testpdnsrecordconfighyphenedwithcount-0.count.sysa.xyz.:::A"}`
	resourceID1 := `{"zone":"count.sysa.xyz.","id":"testpdnsrecordconfighyphenedwithcount-1.count.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigHyphenedWithCount,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists("powerdns_record.test-counted.0"),
					testAccCheckPDNSRecordExists("powerdns_record.test-counted.1"),
				),
			},
			{
				ResourceName:      "powerdns_record.test-counted[0]",
				ImportStateId:     resourceID0,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "powerdns_record.test-counted[1]",
				ImportStateId:     resourceID1,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_AAAA(t *testing.T) {
	resourceName := "powerdns_record.test-aaaa"
	resourceID := `{"zone":"aaaa.sysa.xyz.","id":"testpdnsrecordconfigaaaa.aaaa.sysa.xyz.:::AAAA"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigAAAA,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_CNAME(t *testing.T) {
	resourceName := "powerdns_record.test-cname"
	resourceID := `{"zone":"cname.sysa.xyz.","id":"testpdnsrecordconfigcname.cname.sysa.xyz.:::CNAME"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigCNAME,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_HINFO(t *testing.T) {
	resourceName := "powerdns_record.test-hinfo"
	resourceID := `{"zone":"hinfo.sysa.xyz.","id":"testpdnsrecordconfighinfo.hinfo.sysa.xyz.:::HINFO"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigHINFO,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_LOC(t *testing.T) {
	resourceName := "powerdns_record.test-loc"
	resourceID := `{"zone":"loc.sysa.xyz.","id":"testpdnsrecordconfigloc.loc.sysa.xyz.:::LOC"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigLOC,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					testAccCheckPDNSZoneExists("powerdns_zone.test-zone"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_MX(t *testing.T) {
	resourceName := "powerdns_record.test-mx"
	resourceNameMulti := "powerdns_record.test-mx-multi"
	resourceID := `{"zone":"mx1.sysa.xyz.","id":"mx1.sysa.xyz.:::MX"}`
	resourceIDMulti := `{"zone":"mx2.sysa.xyz.","id":"multi.mx2.sysa.xyz.:::MX"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigMX,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testPDNSRecordConfigMXMulti,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceNameMulti),
				),
			},
			{
				ResourceName:      resourceNameMulti,
				ImportStateId:     resourceIDMulti,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_NAPTR(t *testing.T) {
	resourceName := "powerdns_record.test-naptr"
	resourceID := `{"zone":"naptr.sysa.xyz.","id":"naptr.sysa.xyz.:::NAPTR"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigNAPTR,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_NS(t *testing.T) {
	resourceName := "powerdns_record.test-ns"
	resourceID := `{"zone":"ns.sysa.xyz.","id":"lab.ns.sysa.xyz.:::NS"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigNS,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_SPF(t *testing.T) {
	resourceName := "powerdns_record.test-spf"
	resourceID := `{"zone":"spf.sysa.xyz.","id":"spf.sysa.xyz.:::SPF"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigSPF,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_SSHFP(t *testing.T) {
	resourceName := "powerdns_record.test-sshfp"
	resourceID := `{"zone":"sshfp.sysa.xyz.","id":"ssh.sshfp.sysa.xyz.:::SSHFP"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigSSHFP,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					testAccCheckPDNSZoneExists("powerdns_zone.test-sshfp-zone"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_SRV(t *testing.T) {
	resourceName := "powerdns_record.test-srv"
	resourceID := `{"zone":"srv.sysa.xyz.","id":"_redis._tcp.srv.sysa.xyz.:::SRV"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigSRV,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_TXT(t *testing.T) {
	resourceName := "powerdns_record.test-txt"
	resourceID := `{"zone":"txt.sysa.xyz.","id":"text.txt.sysa.xyz.:::TXT"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigTXT,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_ALIAS(t *testing.T) {
	resourceName := "powerdns_record.test-alias"
	resourceID := `{"zone":"alias.sysa.xyz.","id":"alias.alias.sysa.xyz.:::ALIAS"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigALIAS,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSRecord_SOA(t *testing.T) {
	resourceName := "powerdns_record.test-soa"
	resourceID := `{"zone":"test-soa-sysa.xyz.","id":"test-soa-sysa.xyz.:::SOA"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigSOA,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportStateId:     resourceID,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckPDNSRecordDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_record" {
			continue
		}

		client := testAccProvider.Meta().(*Client)
		exists, err := client.RecordExistsByID(rs.Primary.Attributes["zone"], rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("Error checking if record still exists: %#v", rs.Primary.ID)
		}
		if exists {
			return fmt.Errorf("Record still exists: %#v", rs.Primary.ID)
		}

	}
	return nil
}

func testAccCheckPDNSRecordExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Record ID is set")
		}

		client := testAccProvider.Meta().(*Client)
		foundRecords, err := client.ListRecordsByID(rs.Primary.Attributes["zone"], rs.Primary.ID)
		if err != nil {
			return err
		}
		if len(foundRecords) == 0 {
			return fmt.Errorf("Record does not exist")
		}
		for _, rec := range foundRecords {
			if rec.ID() == rs.Primary.ID {
				return nil
			}
		}
		return fmt.Errorf("Record does not exist: %#v", rs.Primary.ID)
	}
}

const testPDNSRecordConfigRecordEmpty = `
resource "powerdns_record" "test-a" {
	zone = "sysa.xyz."
	name = "testpdnsrecordconfigrecordempty.sysa.xyz."
	type = "A"
	ttl = 60
	records = [ ]
}`

const testPDNSRecordConfigA = `
resource "powerdns_zone" "test-a-zone" {
	name = "a.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-a" {
	zone = powerdns_zone.test-a-zone.name
	name = "testpdnsrecordconfiga.a.sysa.xyz."
	type = "A"
	ttl = 60
	records = [ "1.1.1.1", "2.2.2.2" ]
}`

const testPDNSRecordConfigAWithPtr = `
resource "powerdns_zone" "test-ptr-zone" {
	name = "ptr.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-a-ptr" {
	zone = powerdns_zone.test-ptr-zone.name
	name = "testpdnsrecordconfigawithptr.ptr.sysa.xyz."
	type = "A"
	ttl = 60
	set_ptr = true
	records = [ "1.1.1.1" ]
}`

const testPDNSRecordConfigHyphenedWithCount = `
resource "powerdns_zone" "test-count-zone" {
	name = "count.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-counted" {
	count = "2"
	zone = powerdns_zone.test-count-zone.name
	name = "testpdnsrecordconfighyphenedwithcount-${count.index}.count.sysa.xyz."
	type = "A"
	ttl = 60
	records = [ "1.1.1.${count.index}" ]
}`

const testPDNSRecordConfigAAAA = `
resource "powerdns_zone" "test-aaaa-zone" {
	name = "aaaa.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-aaaa" {
	zone = powerdns_zone.test-aaaa-zone.name
	name = "testpdnsrecordconfigaaaa.aaaa.sysa.xyz."
	type = "AAAA"
	ttl = 60
	records = [ "2001:db8:2000:bf0::1", "2001:db8:2000:bf1::1" ]
}`

const testPDNSRecordConfigCNAME = `
resource "powerdns_zone" "test-cname-zone" {
	name = "cname.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-cname" {
	zone = powerdns_zone.test-cname-zone.name
	name = "testpdnsrecordconfigcname.cname.sysa.xyz."
	type = "CNAME"
	ttl = 60
	records = [ "redis.example.com." ]
}`

const testPDNSRecordConfigHINFO = `
resource "powerdns_zone" "test-hinfo-zone" {
	name = "hinfo.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-hinfo" {
	zone = powerdns_zone.test-hinfo-zone.name
	name = "testpdnsrecordconfighinfo.hinfo.sysa.xyz."
	type = "HINFO"
	ttl = 60
	records = [ "\"PC-Intel-2.4ghz\" \"Linux\"" ]
}`

const testPDNSRecordConfigLOC = `
resource "powerdns_zone" "test-zone" {
	name = "loc.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-loc" {
	zone = powerdns_zone.test-zone.name
	name = "testpdnsrecordconfigloc.loc.sysa.xyz."
	type = "LOC"
	ttl = 60
	records = [ "51 56 0.123 N 5 54 0.000 E 4.00m 1.00m 10000.00m 10.00m" ]
}`

const testPDNSRecordConfigMX = `
resource "powerdns_zone" "test-mx-zone" {
	name = "mx1.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-mx" {
	zone = powerdns_zone.test-mx-zone.name
	name = "mx1.sysa.xyz."
	type = "MX"
	ttl = 60
	records = [ "10 mail.example.com." ]
}`

const testPDNSRecordConfigMXMulti = `
resource "powerdns_zone" "test-mx-multi-zone" {
	name = "mx2.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-mx-multi" {
	zone = powerdns_zone.test-mx-multi-zone.name
	name = "multi.mx2.sysa.xyz."
	type = "MX"
	ttl = 60
	records = [ "10 mail1.example.com.", "20 mail2.example.com." ]
}`

const testPDNSRecordConfigNAPTR = `
resource "powerdns_zone" "test-naptr-zone" {
	name = "naptr.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-naptr" {
	zone = powerdns_zone.test-naptr-zone.name
	name = "naptr.sysa.xyz."
	type = "NAPTR"
	ttl = 60
	records = [ "100 50 \"s\" \"z3950+I2L+I2C\" \"\" _z3950._tcp.gatech.edu'." ]
}`

const testPDNSRecordConfigNS = `
resource "powerdns_zone" "test-ns-zone" {
	name = "ns.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-ns" {
	zone = powerdns_zone.test-ns-zone.name
	name = "lab.ns.sysa.xyz."
	type = "NS"
	ttl = 60
	records = [ "ns1.ns.sysa.xyz.", "ns2.ns.sysa.xyz." ]
}`

const testPDNSRecordConfigSPF = `
resource "powerdns_zone" "test-spf-zone" {
	name = "spf.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-spf" {
	zone = powerdns_zone.test-spf-zone.name
	name = "spf.sysa.xyz."
	type = "SPF"
	ttl = 60
	records = [ "\"v=spf1 +all\"" ]
}`

const testPDNSRecordConfigSSHFP = `
resource "powerdns_zone" "test-sshfp-zone" {
	name = "sshfp.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-sshfp" {
	zone = powerdns_zone.test-sshfp-zone.name
	name = "ssh.sshfp.sysa.xyz."
	type = "SSHFP"
	ttl = 60
	records = [ "1 1 123456789abcdef67890123456789abcdef67890" ]
}`

const testPDNSRecordConfigSRV = `
resource "powerdns_zone" "test-srv-zone" {
	name = "srv.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-srv" {
	zone = powerdns_zone.test-srv-zone.name
	name = "_redis._tcp.srv.sysa.xyz."
	type = "SRV"
	ttl = 60
	records = [ "0 10 6379 redis1.srv.sysa.xyz.", "0 10 6379 redis2.srv.sysa.xyz.", "10 10 6379 redis-replica.srv.sysa.xyz." ]
}`

const testPDNSRecordConfigTXT = `
resource "powerdns_zone" "test-txt-zone" {
	name = "txt.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-txt" {
	zone = powerdns_zone.test-txt-zone.name
	name = "text.txt.sysa.xyz."
	type = "TXT"
	ttl = 60
	records = [ "\"text record payload\"" ]
}`

const testPDNSRecordConfigALIAS = `
resource "powerdns_zone" "test-alias-zone" {
	name = "alias.sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-alias" {
	zone = powerdns_zone.test-alias-zone.name
	name = "alias.alias.sysa.xyz."
	type = "ALIAS"
	ttl = 3600
	records = [ "www.some-alias.com." ]
}`

const testPDNSRecordConfigSOA = `
resource "powerdns_zone" "test-soa-zone" {
	name = "test-soa-sysa.xyz."
	kind = "Native"
	nameservers = ["ns1.sysa.xyz.", "ns2.sysa.xyz."]
}

resource "powerdns_record" "test-soa" {
	zone = powerdns_zone.test-soa-zone.name
	name = powerdns_zone.test-soa-zone.name
	type = "SOA"
	ttl = 3600
	records = [ "something.something. hostmaster.sysa.xyz. 2019090301 10800 3600 604800 3600" ]
}`
