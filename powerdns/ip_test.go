package powerdns

import "testing"

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		expectError bool
	}{
		// IPv4 test cases
		{
			name:        "Valid IPv4 /8 CIDR",
			cidr:        "10.0.0.0/8",
			expectError: false,
		},
		{
			name:        "Valid IPv4 /16 CIDR",
			cidr:        "172.16.0.0/16",
			expectError: false,
		},
		{
			name:        "Valid IPv4 /24 CIDR",
			cidr:        "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "Invalid IPv4 CIDR - wrong prefix length",
			cidr:        "10.0.0.0/12",
			expectError: true,
		},
		{
			name:        "Invalid IPv4 CIDR - invalid IP",
			cidr:        "256.0.0.0/8",
			expectError: true,
		},
		{
			name:        "Invalid IPv4 CIDR - invalid format",
			cidr:        "10.0.0.0",
			expectError: true,
		},

		// IPv6 test cases
		{
			name:        "Valid IPv6 /4 CIDR",
			cidr:        "2000::/4",
			expectError: false,
		},
		{
			name:        "Valid IPv6 /8 CIDR",
			cidr:        "2001::/8",
			expectError: false,
		},
		{
			name:        "Valid IPv6 /12 CIDR",
			cidr:        "2001:db8::/12",
			expectError: false,
		},
		{
			name:        "Valid IPv6 /16 CIDR",
			cidr:        "2001:db8::/16",
			expectError: false,
		},
		{
			name:        "Valid IPv6 /124 CIDR",
			cidr:        "2001:db8::/124",
			expectError: false,
		},
		{
			name:        "Invalid IPv6 CIDR - prefix length too small",
			cidr:        "2001::/3",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 CIDR - prefix length too large",
			cidr:        "2001::/125",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 CIDR - prefix length not multiple of 4",
			cidr:        "2001::/10",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 CIDR - invalid IP",
			cidr:        "2001:g::/8",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 CIDR - invalid format",
			cidr:        "2001::",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ValidateCIDR(tt.cidr, "cidr")
			if tt.expectError {
				if len(errs) == 0 {
					t.Errorf("validateCIDR() expected error but got none")
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("validateCIDR() unexpected error: %v", errs)
				}
			}
		})
	}
}

func TestGetPTRRecordName(t *testing.T) {
	testCases := []struct {
		name        string
		ip          string
		expected    string
		expectError bool
	}{
		{
			name:     "Valid IPv4 address",
			ip:       "10.1.2.3",
			expected: "3.2.1.10",
		},
		{
			name:     "Loopback address",
			ip:       "127.0.0.1",
			expected: "1.0.0.127",
		},
		{
			name:     "Zero address",
			ip:       "0.0.0.0",
			expected: "0.0.0.0",
		},
		{
			name:     "Valid IPv6 address",
			ip:       "2001:db8::1",
			expected: "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2",
		},
		{
			name:     "IPv6 with multiple segments",
			ip:       "2001:db8:1:2:3:4:5:6",
			expected: "6.0.0.0.5.0.0.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.8.b.d.0.1.0.0.2",
		},
		{
			name:     "IPv6 with leading zeros",
			ip:       "2001:0db8:0000:0000:0000:0000:0000:0001",
			expected: "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2",
		},
		{
			name:        "Invalid IPv4 address",
			ip:          "256.1.2.3",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 address",
			ip:          "2001:db8::1::2",
			expectError: true,
		},
		{
			name:        "Invalid IP format",
			ip:          "not.an.ip",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetPTRRecordName(tc.ip)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for IP %s, but got none", tc.ip)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for IP %s: %v", tc.ip, err)
				}
				if result != tc.expected {
					t.Errorf("For IP %s, expected PTR name %s, but got %s", tc.ip, tc.expected, result)
				}
			}
		})
	}
}

func TestParsePTRRecordName(t *testing.T) {
	tests := []struct {
		name        string
		ptrName     string
		expectedIP  string
		expectError bool
	}{
		// IPv4 test cases
		{
			name:        "Valid IPv4 PTR record",
			ptrName:     "3.2.1.10.in-addr.arpa.",
			expectedIP:  "10.1.2.3",
			expectError: false,
		},
		{
			name:        "Valid IPv4 PTR record with single digit octets",
			ptrName:     "1.0.0.127.in-addr.arpa.",
			expectedIP:  "127.0.0.1",
			expectError: false,
		},
		{
			name:        "Invalid IPv4 PTR record - wrong number of octets",
			ptrName:     "1.2.3.in-addr.arpa.",
			expectedIP:  "",
			expectError: true,
		},
		{
			name:        "Invalid IPv4 PTR record - invalid octet",
			ptrName:     "3.2.1.256.in-addr.arpa.",
			expectedIP:  "",
			expectError: true,
		},
		{
			name:        "Invalid IPv4 PTR record - wrong suffix",
			ptrName:     "3.2.1.10.example.com.",
			expectedIP:  "",
			expectError: true,
		},

		// IPv6 test cases
		{
			name:        "Valid IPv6 PTR record",
			ptrName:     "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			expectedIP:  "2001:db8::1",
			expectError: false,
		},
		{
			name:        "Valid IPv6 PTR record with full address",
			ptrName:     "6.0.0.0.5.0.0.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			expectedIP:  "2001:db8:1:2:3:4:5:6",
			expectError: false,
		},
		{
			name:        "Invalid IPv6 PTR record - wrong number of nibbles",
			ptrName:     "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.ip6.arpa.",
			expectedIP:  "",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 PTR record - invalid nibble",
			ptrName:     "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2g.ip6.arpa.",
			expectedIP:  "",
			expectError: true,
		},
		{
			name:        "Invalid IPv6 PTR record - wrong suffix",
			ptrName:     "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.example.com.",
			expectedIP:  "",
			expectError: true,
		},

		// Invalid format test cases
		{
			name:        "Invalid PTR record - no suffix",
			ptrName:     "1.2.3.4",
			expectedIP:  "",
			expectError: true,
		},
		{
			name:        "Invalid PTR record - empty string",
			ptrName:     "",
			expectedIP:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := ParsePTRRecordName(tt.ptrName)
			if tt.expectError {
				if err == nil {
					t.Errorf("parsePTRRecordName() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parsePTRRecordName() unexpected error: %v", err)
				}
				if ip.String() != tt.expectedIP {
					t.Errorf("parsePTRRecordName() = %v, want %v", ip.String(), tt.expectedIP)
				}
			}
		})
	}
}

func TestParseReverseZoneName(t *testing.T) {
	tests := []struct {
		name         string
		zoneName     string
		expectedCIDR string
		expectError  bool
	}{
		// IPv4 test cases
		{
			name:         "Valid IPv4 /8 zone",
			zoneName:     "10.in-addr.arpa.",
			expectedCIDR: "10.0.0.0/8",
			expectError:  false,
		},
		{
			name:         "Valid IPv4 /16 zone",
			zoneName:     "16.172.in-addr.arpa.",
			expectedCIDR: "172.16.0.0/16",
			expectError:  false,
		},
		{
			name:         "Valid IPv4 /24 zone",
			zoneName:     "1.168.192.in-addr.arpa.",
			expectedCIDR: "192.168.1.0/24",
			expectError:  false,
		},
		{
			name:         "Valid IPv4 zone without trailing dot",
			zoneName:     "10.in-addr.arpa",
			expectedCIDR: "",
			expectError:  true,
		},
		{
			name:         "Invalid IPv4 zone - too many octets",
			zoneName:     "1.2.3.4.in-addr.arpa.",
			expectedCIDR: "",
			expectError:  true,
		},
		{
			name:         "Invalid IPv4 zone - invalid octet",
			zoneName:     "256.in-addr.arpa.",
			expectedCIDR: "",
			expectError:  true,
		},

		// IPv6 test cases
		{
			name:         "Valid IPv6 /4 zone",
			zoneName:     "2.ip6.arpa.",
			expectedCIDR: "2000::/4",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /8 zone",
			zoneName:     "0.2.ip6.arpa.",
			expectedCIDR: "2000::/8",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /12 zone",
			zoneName:     "0.0.2.ip6.arpa.",
			expectedCIDR: "2000::/12",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /16 zone",
			zoneName:     "1.0.0.2.ip6.arpa.",
			expectedCIDR: "2001::/16",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 /124 zone",
			zoneName:     "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			expectedCIDR: "2001:db8::/124",
			expectError:  false,
		},
		{
			name:         "Valid IPv6 zone without trailing dot",
			zoneName:     "2.ip6.arpa",
			expectedCIDR: "",
			expectError:  true,
		},
		{
			name:         "Invalid IPv6 zone - too many nibbles",
			zoneName:     "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.",
			expectedCIDR: "",
			expectError:  true,
		},
		{
			name:         "Invalid IPv6 zone - invalid nibble",
			zoneName:     "g.ip6.arpa.",
			expectedCIDR: "",
			expectError:  true,
		},

		// General invalid cases
		{
			name:         "Invalid zone - wrong suffix",
			zoneName:     "example.com.",
			expectedCIDR: "",
			expectError:  true,
		},
		{
			name:         "Invalid zone - empty string",
			zoneName:     "",
			expectedCIDR: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cidr, err := ParseReverseZoneName(tt.zoneName)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseReverseZoneName() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ParseReverseZoneName() unexpected error: %v", err)
				}
				if cidr != tt.expectedCIDR {
					t.Errorf("ParseReverseZoneName() = %v, want %v", cidr, tt.expectedCIDR)
				}
			}
		})
	}
}
