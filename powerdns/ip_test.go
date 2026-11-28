package powerdns

import (
	"testing"
)

func TestValidateViewName(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{name: "Valid lowercase", value: "internal", expectError: false},
		{name: "Valid uppercase", value: "Internal", expectError: false},
		{name: "Valid with dash", value: "internal-view", expectError: false},
		{name: "Valid with dot", value: "internal.view", expectError: false},
		{name: "Valid with underscore", value: "internal_view", expectError: false},
		{name: "Valid with space", value: "internal view", expectError: false},
		{name: "Invalid empty", value: "", expectError: true},
		{name: "Invalid starts with dot", value: ".internal", expectError: true},
		{name: "Invalid starts with space", value: " internal", expectError: true},
		{name: "Invalid slash", value: "internal/view", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := validateViewName(tt.value, "view")
			if tt.expectError && len(errs) == 0 {
				t.Errorf("validateViewName(%q) expected error but got none", tt.value)
			}
			if !tt.expectError && len(errs) > 0 {
				t.Errorf("validateViewName(%q) unexpected error: %v", tt.value, errs)
			}
		})
	}
}

func TestValidateZoneName(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{name: "Valid FQDN", value: "example.com.", expectError: false},
		{name: "Valid variant", value: "example.com..internal", expectError: false},
		{name: "Valid root variant", value: "..internal", expectError: false},
		{name: "Invalid missing trailing dot", value: "example.com", expectError: true},
		{name: "Invalid variant ending with dot", value: "example.com..internal.", expectError: true},
		{name: "Invalid multiple variant separators", value: "example.com..internal..blue", expectError: true},
		{name: "Invalid variant uppercase", value: "example.com..Internal", expectError: true},
		{name: "Invalid base with trailing dot before variant", value: "example.com...internal", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ValidateZoneName(tt.value, "name")
			if tt.expectError && len(errs) == 0 {
				t.Errorf("ValidateZoneName(%q) expected error but got none", tt.value)
			}
			if !tt.expectError && len(errs) > 0 {
				t.Errorf("ValidateZoneName(%q) unexpected error: %v", tt.value, errs)
			}
		})
	}
}

func TestValidateFQDN(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{
			name:        "Valid FQDN with trailing dot",
			value:       "example.com.",
			expectError: false,
		},
		{
			name:        "Valid subdomain FQDN",
			value:       "sub.example.com.",
			expectError: false,
		},
		{
			name:        "Valid deeply nested FQDN",
			value:       "a.b.c.d.example.com.",
			expectError: false,
		},
		{
			name:        "Root zone",
			value:       ".",
			expectError: false,
		},
		{
			name:        "Missing trailing dot",
			value:       "example.com",
			expectError: true,
		},
		{
			name:        "Missing trailing dot on subdomain",
			value:       "sub.example.com",
			expectError: true,
		},
		{
			name:        "Empty string",
			value:       "",
			expectError: true,
		},
		{
			name:        "Single label without dot",
			value:       "localhost",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ValidateFQDN(tt.value, "test_field")
			if tt.expectError {
				if len(errs) == 0 {
					t.Errorf("ValidateFQDN(%q) expected error but got none", tt.value)
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("ValidateFQDN(%q) unexpected error: %v", tt.value, errs)
				}
			}
		})
	}
}

func TestValidateMasterAddress(t *testing.T) {
	valid := []string{
		"192.0.2.1",
		"2001:db8::1",
		"fd92:81e1:e314:ea7b:0000:1234:5678:60ab",
		"::1",
		"192.0.2.1:53",
		"[2001:db8::1]:53",
		"192.0.2.1:1",
		"192.0.2.1:65535",
	}
	for _, value := range valid {
		if _, errs := ValidateMasterAddress(value, "masters"); len(errs) != 0 {
			t.Errorf("%q should be a valid master, got: %v", value, errs)
		}
	}

	invalid := []string{
		"",
		"ns1.example.com",
		"ns1.example.com:53",
		"192.0.2.1:0",
		"192.0.2.1:65536",
		"192.0.2.1:dns",
		"999.0.2.1",
		"not an address",
	}
	for _, value := range invalid {
		if _, errs := ValidateMasterAddress(value, "masters"); len(errs) == 0 {
			t.Errorf("%q should not be a valid master", value)
		}
	}
}

func TestNormalizeMasterAddress(t *testing.T) {
	tests := map[string]struct {
		value    string
		expected string
	}{
		"IPv4":           {value: "192.0.2.1", expected: "192.0.2.1"},
		"IPv6":           {value: "fd92:81e1:e314:ea7b:0000:1234:5678:60ab", expected: "fd92:81e1:e314:ea7b:0:1234:5678:60ab"},
		"IPv4 with port": {value: "192.0.2.1:53", expected: "192.0.2.1:53"},
		"IPv6 with port": {value: "[fd92:81e1:e314:ea7b:0000:1234:5678:60ab]:53", expected: "[fd92:81e1:e314:ea7b:0:1234:5678:60ab]:53"},
		"invalid":        {value: "example.com", expected: "example.com"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if actual := NormalizeMasterAddress(tt.value); actual != tt.expected {
				t.Errorf("NormalizeMasterAddress(%q) = %q, want %q", tt.value, actual, tt.expected)
			}
		})
	}
}
