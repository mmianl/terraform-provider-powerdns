package powerdns

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccPowerDNSReverseZone_CIDR(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSReverseZoneConfig_CIDR_8,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists("powerdns_reverse_zone.test_8"),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_8", "name", "10.in-addr.arpa."),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_8", "kind", "Master"),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_8", "nameservers.0", "ns1.example.com."),
				),
			},
			{
				Config: testAccPowerDNSReverseZoneConfig_CIDR_16,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists("powerdns_reverse_zone.test_16"),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_16", "name", "16.172.in-addr.arpa."),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_16", "kind", "Master"),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_16", "nameservers.0", "ns1.example.com."),
				),
			},
			{
				Config: testAccPowerDNSReverseZoneConfig_CIDR_24,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists("powerdns_reverse_zone.test_24"),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_24", "name", "24.0.10.in-addr.arpa."),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_24", "kind", "Master"),
					resource.TestCheckResourceAttr(
						"powerdns_reverse_zone.test_24", "nameservers.0", "ns1.example.com."),
				),
			},
		},
	})
}

func TestAccPowerDNSReverseZone_InvalidCIDR(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccPowerDNSReverseZoneConfig_InvalidCIDR,
				ExpectError: regexp.MustCompile("prefix length must be 8, 16, or 24"),
			},
		},
	})
}

func TestAccPowerDNSReverseZone_IPv6(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPowerDNSReverseZoneConfig_IPv6,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists("powerdns_reverse_zone.test_ipv6"),
					resource.TestCheckResourceAttr("powerdns_reverse_zone.test_ipv6", "cidr", "2001:db8::/32"),
					resource.TestCheckResourceAttr("powerdns_reverse_zone.test_ipv6", "kind", "Master"),
					resource.TestCheckResourceAttr("powerdns_reverse_zone.test_ipv6", "name", "8.b.d.0.1.0.0.2.ip6.arpa."),
				),
			},
		},
	})
}

func TestAccPowerDNSReverseZone_InvalidIPv6Prefix(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccPowerDNSReverseZoneConfig_InvalidIPv6Prefix,
				ExpectError: regexp.MustCompile("IPv6 prefix length must be a multiple of 4 between 4 and 124"),
			},
		},
	})
}

func TestGetReverseZoneName(t *testing.T) {
	tests := []struct {
		name         string
		cidr         string
		expectedZone string
		expectError  bool
	}{
		// IPv4 test cases
		{
			name:         "Valid IPv4 /8 CIDR",
			cidr:         "10.0.0.0/8",
			expectedZone: "10.in-addr.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv4 /16 CIDR",
			cidr:         "172.16.0.0/16",
			expectedZone: "16.172.in-addr.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv4 /24 CIDR",
			cidr:         "192.168.1.0/24",
			expectedZone: "1.168.192.in-addr.arpa.",
			expectError:  false,
		},
		{
			name:         "Invalid IPv4 /12 CIDR",
			cidr:         "10.0.0.0/12",
			expectedZone: "10.in-addr.arpa.",
			expectError:  false,
		},

		// IPv6 test cases
		{
			name:         "Valid IPv6 /4 CIDR",
			cidr:         "2000::/4",
			expectedZone: "2.ip6.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /8 CIDR",
			cidr:         "2001::/8",
			expectedZone: "0.2.ip6.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /12 CIDR",
			cidr:         "2001:db8::/12",
			expectedZone: "0.0.2.ip6.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /16 CIDR",
			cidr:         "2001:db8::/16",
			expectedZone: "1.0.0.2.ip6.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /124 CIDR",
			cidr:         "2001:db8::/124",
			expectedZone: "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /10 CIDR",
			cidr:         "2001::/10",
			expectedZone: "0.2.ip6.arpa.",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone, err := getReverseZoneName(tt.cidr)
			if tt.expectError {
				if err == nil {
					t.Errorf("getReverseZoneName() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("getReverseZoneName() unexpected error: %v", err)
				}
				if zone != tt.expectedZone {
					t.Errorf("getReverseZoneName() = %v, want %v", zone, tt.expectedZone)
				}
			}
		})
	}
}

func TestExpandStringList(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []string
	}{
		{
			name:     "Empty list",
			input:    []interface{}{},
			expected: []string{},
		},
		{
			name:     "Single string",
			input:    []interface{}{"ns1.example.com"},
			expected: []string{"ns1.example.com"},
		},
		{
			name:     "Multiple strings",
			input:    []interface{}{"ns1.example.com", "ns2.example.com"},
			expected: []string{"ns1.example.com", "ns2.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandStringList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expandStringList() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expandStringList()[%d] = %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

const testAccPowerDNSReverseZoneConfig_CIDR_8 = `
resource "powerdns_reverse_zone" "test_8" {
  cidr        = "10.0.0.0/8"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}
`

const testAccPowerDNSReverseZoneConfig_CIDR_16 = `
resource "powerdns_reverse_zone" "test_16" {
  cidr        = "172.16.0.0/16"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}
`

const testAccPowerDNSReverseZoneConfig_CIDR_24 = `
resource "powerdns_reverse_zone" "test_24" {
  cidr        = "10.0.24.0/24"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}
`

const testAccPowerDNSReverseZoneConfig_InvalidCIDR = `
resource "powerdns_reverse_zone" "test" {
  cidr        = "172.16.0.0/20"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}
`

const testAccPowerDNSReverseZoneConfig_IPv6 = `
resource "powerdns_reverse_zone" "test_ipv6" {
  cidr        = "2001:db8::/32"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}
`

const testAccPowerDNSReverseZoneConfig_InvalidIPv6Prefix = `
resource "powerdns_reverse_zone" "test_ipv6" {
  cidr        = "2001:db8::/33"
  kind        = "Master"
  nameservers = ["ns1.example.com."]
}
`
