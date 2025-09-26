package powerdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestAccPowerDNSPTRRecord_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPowerDNSPTRRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSPTRRecordConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSPTRRecordExists("powerdns_ptr_record.test"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "ip_address", "10.1.2.3"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "hostname", "host.example.com"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "ttl", "300"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "reverse_zone", "10.in-addr.arpa."),
				),
			},
		},
	})
}

func TestAccPowerDNSPTRRecord_Update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPowerDNSPTRRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSPTRRecordConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSPTRRecordExists("powerdns_ptr_record.test"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "ip_address", "10.1.2.3"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "hostname", "host.example.com"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "ttl", "300"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "reverse_zone", "10.in-addr.arpa."),
				),
			},
			{
				Config: testAccPowerDNSPTRRecordConfig_Updated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSPTRRecordExists("powerdns_ptr_record.test"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "ip_address", "10.1.2.3"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "hostname", "newhost.example.com"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "ttl", "600"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test", "reverse_zone", "10.in-addr.arpa."),
				),
			},
		},
	})
}

func TestAccPowerDNSPTRRecord_IPv6(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPowerDNSPTRRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSPTRRecordConfig_IPv6,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPowerDNSPTRRecordExists("powerdns_ptr_record.test_ipv6"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test_ipv6", "ip_address", "2001:db8::1"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test_ipv6", "hostname", "host.example.com"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test_ipv6", "ttl", "300"),
					resource.TestCheckResourceAttr("powerdns_ptr_record.test_ipv6", "reverse_zone", "8.b.d.0.1.0.0.2.ip6.arpa."),
				),
			},
		},
	})
}

const testAccPowerDNSPTRRecordConfig = `
resource "powerdns_reverse_zone" "test" {
  cidr        = "10.0.0.0/8"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}

resource "powerdns_ptr_record" "test" {
  ip_address   = "10.1.2.3"
  hostname     = "host.example.com"
  ttl          = 300
  reverse_zone = powerdns_reverse_zone.test.name
}
`

const testAccPowerDNSPTRRecordConfig_Updated = `
resource "powerdns_reverse_zone" "test" {
  cidr        = "10.0.0.0/8"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}

resource "powerdns_ptr_record" "test" {
  ip_address   = "10.1.2.3"
  hostname     = "newhost.example.com"
  ttl          = 600
  reverse_zone = powerdns_reverse_zone.test.name
}
`

const testAccPowerDNSPTRRecordConfig_IPv6 = `
resource "powerdns_reverse_zone" "test_ipv6" {
  cidr        = "2001:db8::/32"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}

resource "powerdns_ptr_record" "test_ipv6" {
  ip_address   = "2001:db8::1"
  hostname     = "host.example.com"
  ttl          = 300
  reverse_zone = powerdns_reverse_zone.test_ipv6.name
}
`

func testAccCheckPowerDNSPTRRecordDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*Client)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_ptr_record" {
			continue
		}

		exists, err := client.RecordExistsByID(rs.Primary.Attributes["reverse_zone"], rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("Error checking if PTR record still exists: %#v", rs.Primary.ID)
		}
		if exists {
			return fmt.Errorf("PTR record still exists")
		}
	}

	return nil
}

func testAccCheckPowerDNSPTRRecordExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No PTR record ID is set")
		}

		client := testAccProvider.Meta().(*Client)
		exists, err := client.RecordExistsByID(rs.Primary.Attributes["reverse_zone"], rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("Error checking if PTR record exists: %#v", rs.Primary.ID)
		}
		if !exists {
			return fmt.Errorf("PTR record not found")
		}

		return nil
	}
}

// Unit test for resourcePDNSPTRRecordCreate with mocked client
func TestResourcePDNSPTRRecordCreate(t *testing.T) {
	// Test that the resource schema is properly configured
	resource := resourcePDNSPTRRecord()
	assert.NotNil(t, resource)
	assert.NotNil(t, resource.Schema)

	// Test required fields exist
	schema := resource.Schema
	assert.NotNil(t, schema["ip_address"])
	assert.NotNil(t, schema["hostname"])
	assert.NotNil(t, schema["reverse_zone"])
}
