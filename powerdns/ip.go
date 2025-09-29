package powerdns

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidateCIDR validates the CIDR format.
// For IPv4, only /8, /16, /24 are allowed.
// For IPv6, prefix length must be a multiple of 4 between 4 and 124.
func ValidateCIDR(v interface{}, k string) (ws []string, errors []error) {
	cidr := v.(string)
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		errors = append(errors, fmt.Errorf("invalid CIDR format: %s", err))
		return
	}
	// Normalize to the network address (safety; not strictly needed here)
	ip = ip.Mask(ipnet.Mask)
	_ = ip

	ones, _ := ipnet.Mask.Size()

	// Check if it's an IPv4 or IPv6 CIDR
	if ipnet.IP.To4() != nil {
		// IPv4 CIDR
		if ones != 8 && ones != 16 && ones != 24 {
			errors = append(errors, fmt.Errorf("IPv4 prefix length must be 8, 16, or 24"))
			return
		}
	} else {
		// IPv6 CIDR
		if ones%4 != 0 || ones < 4 || ones > 124 {
			errors = append(errors, fmt.Errorf("IPv6 prefix length must be a multiple of 4 between 4 and 124"))
			return
		}
	}

	return
}

// ParsePTRRecordName converts a PTR record name back to an IP address.
func ParsePTRRecordName(name string) (net.IP, error) {
	if strings.HasSuffix(name, ".in-addr.arpa.") {
		// IPv4 PTR record
		parts := strings.Split(strings.TrimSuffix(name, ".in-addr.arpa."), ".")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid IPv4 PTR record name format: %s", name)
		}
		// Reverse the octets
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		ip := net.ParseIP(strings.Join(parts, "."))
		if ip == nil {
			return nil, fmt.Errorf("invalid IPv4 address in PTR record name: %s", name)
		}
		return ip, nil
	} else if strings.HasSuffix(name, ".ip6.arpa.") {
		// IPv6 PTR record
		parts := strings.Split(strings.TrimSuffix(name, ".ip6.arpa."), ".")
		if len(parts) != 32 {
			return nil, fmt.Errorf("invalid IPv6 PTR record name format: %s", name)
		}
		// Convert nibbles back to IPv6 address
		ipv6 := make([]byte, 16)
		for i := 0; i < 32; i += 2 {
			n1, err := strconv.ParseUint(parts[31-i], 16, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid IPv6 nibble in PTR record name: %s", parts[31-i])
			}
			n2, err := strconv.ParseUint(parts[30-i], 16, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid IPv6 nibble in PTR record name: %s", parts[30-i])
			}
			ipv6[i/2] = byte(n1<<4 | n2)
		}
		return net.IP(ipv6), nil
	}

	return nil, fmt.Errorf("unsupported PTR record name format: %s", name)
}

// GetPTRRecordName determines the PTR record name (label portion) based on the IP address.
// It returns the reversed labels without the trailing zone suffix (e.g., "4.3.2.1" or "b.a.9.8....").
func GetPTRRecordName(ip string) (string, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	if ipv4 := parsedIP.To4(); ipv4 != nil {
		// IPv4 PTR record
		ipParts := strings.Split(ipv4.String(), ".")
		if len(ipParts) != 4 {
			return "", fmt.Errorf("invalid IPv4 address: %s", ip)
		}
		// Build the PTR record name using all octets in reverse order
		ptrParts := []string{ipParts[3], ipParts[2], ipParts[1], ipParts[0]}
		return strings.Join(ptrParts, "."), nil
	}

	// IPv6 PTR record
	ipv6 := parsedIP.To16()
	if ipv6 == nil {
		return "", fmt.Errorf("invalid IPv6 address: %s", ip)
	}

	// Convert each byte to two nibbles in reverse order
	ptrParts := make([]string, 32)
	for i := 0; i < 16; i++ {
		b := ipv6[15-i]                                  // reverse byte order
		ptrParts[i*2] = fmt.Sprintf("%x", b&0x0F)        // lower nibble
		ptrParts[i*2+1] = fmt.Sprintf("%x", (b>>4)&0x0F) // higher nibble
	}
	return strings.Join(ptrParts, "."), nil
}

// ParseReverseZoneName converts a reverse zone FQDN to its corresponding CIDR.
// Examples:
//
//	"16.172.in-addr.arpa."   -> "172.16.0.0/16"
//	"b.a.9.8.ip6.arpa." (etc)-> "<ipv6-prefix>/8" (nibble * 4)
func ParseReverseZoneName(name string) (string, error) {
	if strings.HasSuffix(name, ".in-addr.arpa.") {
		// IPv4 reverse zone
		parts := strings.Split(strings.TrimSuffix(name, ".in-addr.arpa."), ".")
		if len(parts) < 1 || len(parts) > 3 {
			return "", fmt.Errorf("invalid IPv4 reverse zone name: %s", name)
		}

		// Convert octets to IP address
		ipParts := make([]string, 4)
		for i := 0; i < 4; i++ {
			if i < len(parts) {
				// Parse and validate octet
				octet, err := strconv.ParseUint(parts[len(parts)-1-i], 10, 8)
				if err != nil || octet > 255 {
					return "", fmt.Errorf("invalid IPv4 octet in zone name: %s", parts[len(parts)-1-i])
				}
				ipParts[i] = fmt.Sprintf("%d", octet)
			} else {
				ipParts[i] = "0"
			}
		}
		ip := strings.Join(ipParts, ".")
		prefixLen := len(parts) * 8
		return fmt.Sprintf("%s/%d", ip, prefixLen), nil
	} else if strings.HasSuffix(name, ".ip6.arpa.") {
		// IPv6 reverse zone
		parts := strings.Split(strings.TrimSuffix(name, ".ip6.arpa."), ".")
		if len(parts) < 1 || len(parts) > 32 {
			return "", fmt.Errorf("invalid IPv6 reverse zone name: %s", name)
		}

		// Convert nibbles to IP address
		ipBytes := make([]byte, 16)
		for i := 0; i < len(parts); i++ {
			byteIndex := i / 2
			nibbleIndex := i % 2
			nibble, err := strconv.ParseUint(parts[len(parts)-1-i], 16, 8)
			if err != nil {
				return "", fmt.Errorf("invalid IPv6 nibble in zone name: %s", parts[len(parts)-1-i])
			}
			if nibbleIndex == 0 {
				ipBytes[byteIndex] = byte(nibble << 4)
			} else {
				ipBytes[byteIndex] |= byte(nibble)
			}
		}

		// Create IPv6 address
		ip := net.IP(ipBytes)
		prefixLen := len(parts) * 4
		// Ensure prefix length is a multiple of 4 and within valid range
		if prefixLen < 4 || prefixLen > 124 || prefixLen%4 != 0 {
			return "", fmt.Errorf("invalid IPv6 prefix length: %d", prefixLen)
		}
		return fmt.Sprintf("%s/%d", ip.String(), prefixLen), nil
	}

	return "", fmt.Errorf("invalid reverse zone name: %s", name)
}

// GetReverseZoneName computes the reverse zone FQDN from a CIDR.
// Examples:

// 10.0.0.0/8    -> 10.in-addr.arpa.
// 172.16.0.0/16 -> 16.172.in-addr.arpa.
// 2000::/4      -> 2.ip6.arpa.
func GetReverseZoneName(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %s", err)
	}

	ones, _ := ipnet.Mask.Size()
	if ipnet.IP.To4() != nil {
		// IPv4 reverse zone
		ip := ipnet.IP.To4()
		octets := ones / 8

		// For /24 networks, we need to include the third octet in the zone name
		if ones == 24 {
			octets = 3
		}

		// Build the zone name based on the number of octets
		zoneParts := make([]string, octets)
		for i := 0; i < octets; i++ {
			zoneParts[i] = fmt.Sprintf("%d", ip[octets-1-i])
		}
		zone := strings.Join(zoneParts, ".") + ".in-addr.arpa."
		return zone, nil
	}

	// IPv6 reverse zone
	ip := ipnet.IP.To16()
	nibbles := ones / 4

	// Build the zone name based on the number of nibbles
	zoneParts := make([]string, nibbles)
	for i := 0; i < nibbles; i++ {
		// Get the nibble (4 bits) from the IP address
		byteIndex := i / 2
		nibbleIndex := i % 2
		byte := ip[byteIndex]
		if nibbleIndex == 0 {
			byte = byte >> 4
		} else {
			byte = byte & 0x0F
		}
		zoneParts[nibbles-1-i] = fmt.Sprintf("%x", byte)
	}
	zone := strings.Join(zoneParts, ".") + ".ip6.arpa."
	return zone, nil

}
