package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// --- Config / helpers -------------------------------------------------------

func authBaseURL() string {
	if v := os.Getenv("PDNS_SERVER_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://pdns:8081"
}

func recursorBaseURL() string {
	if v := os.Getenv("PDNS_RECURSOR_SERVER_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://recursor:8082"
}

func apiKey() string {
	if v := os.Getenv("PDNS_API_KEY"); v != "" {
		return v
	}
	return "testapikey"
}

func newRequest(t *testing.T, method, base, path string) *http.Request {
	t.Helper()

	url := base + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("failed to create request %s %s: %v", method, url, err)
	}
	req.Header.Set("X-API-Key", apiKey())
	req.Header.Set("Accept", "application/json")
	return req
}

func doJSON(t *testing.T, req *http.Request, v interface{}) *http.Response {
	t.Helper()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		t.Fatalf("unexpected status %d for %s %s", resp.StatusCode, req.Method, req.URL)
	}

	if v != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("failed to decode JSON for %s %s: %v", req.Method, req.URL, err)
		}
	}

	return resp
}

// --- Types matching PowerDNS / Recursor APIs --------------------------------

// Authoritative zone (subset)
type authZone struct {
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Masters     []string    `json:"masters"`
	Nameservers []string    `json:"nameservers"`
	RRSets      []authRRSet `json:"rrsets"`
	Records     []authRRSet `json:"records"` // v0 vs v1 compatibility (we only use rrsets)
}

type authRRSet struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	TTL     int          `json:"ttl"`
	Records []authRecord `json:"records"`
}

type authRecord struct {
	Content string `json:"content"`
}

type zoneMetadata struct {
	Kind     string   `json:"kind"`
	Metadata []string `json:"metadata"`
}

// Recursor config setting
type recursorConfig struct {
	Name  string   `json:"name"`
	Value []string `json:"value"`
}

// Recursor forward zone (subset)
type recursorForwardZone struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Kind             string   `json:"kind"`
	Servers          []string `json:"servers"`
	RecursionDesired bool     `json:"recursion_desired"`
}

// --- Helper logic for reverse DNS -------------------------------------------

func ipv4ReverseZoneName(t *testing.T, cidr string) string {
	t.Helper()

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("invalid CIDR %q: %v", cidr, err)
	}

	ip := ipNet.IP.To4()
	if ip == nil {
		t.Fatalf("CIDR %q is not IPv4", cidr)
	}

	ones, _ := ipNet.Mask.Size()
	if ones%8 != 0 {
		t.Fatalf("CIDR %q has non-octet mask (only /8, /16, /24 supported here)", cidr)
	}
	octets := ones / 8

	parts := []string{}
	for i := 0; i < octets; i++ {
		parts = append(parts, strconv.Itoa(int(ip[octets-1-i])))
	}
	return strings.Join(parts, ".") + ".in-addr.arpa."
}

func ipv4PtrName(ipStr string) string {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return ""
	}
	return strconv.Itoa(int(ip[3])) + "." +
		strconv.Itoa(int(ip[2])) + "." +
		strconv.Itoa(int(ip[1])) + "." +
		strconv.Itoa(int(ip[0])) + ".in-addr.arpa."
}

func valuesToSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, v := range values {
		set[v] = true
	}
	return set
}

func assertMetadataKindValues(t *testing.T, metadataByKind map[string][]string, kind string, want []string) {
	t.Helper()

	values, ok := metadataByKind[kind]
	if !ok {
		t.Fatalf("zone metadata %s not found in metadata list", kind)
	}

	valueSet := valuesToSet(values)
	for _, w := range want {
		if !valueSet[w] {
			t.Fatalf("%s metadata missing value %q in %v", kind, w, values)
		}
	}
}

func assertMetadataValues(t *testing.T, got []string, label string, want []string) {
	t.Helper()

	gotSet := valuesToSet(got)
	for _, w := range want {
		if !gotSet[w] {
			t.Fatalf("%s missing value %q in %v", label, w, got)
		}
	}
}

// --- Tests ------------------------------------------------------------------

// Test the authoritative PowerDNS resources created by Terraform.
func TestPowerDNSAuthoritativeResources(t *testing.T) {
	base := authBaseURL()

	// 1) Forward zone: test.example.com.
	{
		req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/zones/test.example.com.")
		var zone authZone
		doJSON(t, req, &zone)

		if zone.Name != "test.example.com." {
			t.Fatalf("zone name: got %q, want %q", zone.Name, "test.example.com.")
		}
		if zone.Kind != "Native" {
			t.Fatalf("zone kind: got %q, want %q", zone.Kind, "Native")
		}

		// Check A record host01.test.example.com.
		var foundA bool
		for _, rrset := range zone.RRSets {
			if rrset.Name == "host01.test.example.com." && rrset.Type == "A" {
				foundA = true
				if rrset.TTL != 30 {
					t.Fatalf("A record TTL: got %d, want 30", rrset.TTL)
				}
				if len(rrset.Records) == 0 {
					t.Fatalf("A record has no records")
				}
				if rrset.Records[0].Content != "172.16.0.10" {
					t.Fatalf("A record content: got %q, want %q", rrset.Records[0].Content, "172.16.0.10")
				}
				break
			}
		}
		if !foundA {
			t.Fatalf("A record host01.test.example.com. not found in zone")
		}

		// Check SOA record test.example.com.
		var foundSOA bool
		for _, rrset := range zone.RRSets {
			if rrset.Name == "test.example.com." && rrset.Type == "SOA" {
				foundSOA = true
				if rrset.TTL != 3600 {
					t.Fatalf("SOA record TTL: got %d, want 3600", rrset.TTL)
				}
				if len(rrset.Records) == 0 {
					t.Fatalf("SOA record has no records")
				}
				content := rrset.Records[0].Content
				// Verify mname and rname are present in the SOA content
				if !strings.Contains(content, "dns1.test.example.com.") {
					t.Fatalf("SOA record content missing mname: got %q", content)
				}
				if !strings.Contains(content, "hostmaster.test.example.com.") {
					t.Fatalf("SOA record content missing rname: got %q", content)
				}
				break
			}
		}
		if !foundSOA {
			t.Fatalf("SOA record test.example.com. not found in zone")
		}
	}

	// 2) Forward zone: test2.example.com. (no soa_edit_api)
	{
		req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/zones/test2.example.com.")
		var zone authZone
		doJSON(t, req, &zone)

		if zone.Name != "test2.example.com." {
			t.Fatalf("zone name: got %q, want %q", zone.Name, "test2.example.com.")
		}
		if zone.Kind != "Native" {
			t.Fatalf("zone kind: got %q, want %q", zone.Kind, "Native")
		}
	}

	// 3) SOA record: verify the SOA record can be read back
	//    via the API and matches the values set in main.tf.
	{
		req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/zones/test.example.com.")
		var zone authZone
		doJSON(t, req, &zone)

		var foundSOA bool
		for _, rrset := range zone.RRSets {
			if rrset.Name == "test.example.com." && rrset.Type == "SOA" {
				foundSOA = true
				content := rrset.Records[0].Content

				// Parse SOA content: "mname rname serial refresh retry expire minimum"
				parts := strings.Fields(content)
				if len(parts) != 7 {
					t.Fatalf("SOA content: expected 7 fields, got %d: %q", len(parts), content)
				}

				mname, rname := parts[0], parts[1]
				refresh, _ := strconv.Atoi(parts[3])
				retry, _ := strconv.Atoi(parts[4])
				expire, _ := strconv.Atoi(parts[5])
				minimum, _ := strconv.Atoi(parts[6])

				if mname != "dns1.test.example.com." {
					t.Fatalf("SOA mname: got %q, want %q", mname, "dns1.test.example.com.")
				}
				if rname != "hostmaster.test.example.com." {
					t.Fatalf("SOA rname: got %q, want %q", rname, "hostmaster.test.example.com.")
				}
				if refresh != 10800 {
					t.Fatalf("SOA refresh: got %d, want 10800", refresh)
				}
				if retry != 3600 {
					t.Fatalf("SOA retry: got %d, want 3600", retry)
				}
				if expire != 3600000 {
					t.Fatalf("SOA expire: got %d, want 3600000", expire)
				}
				if minimum != 3600 {
					t.Fatalf("SOA minimum: got %d, want 3600", minimum)
				}
				if rrset.TTL != 3600 {
					t.Fatalf("SOA TTL: got %d, want 3600", rrset.TTL)
				}
				break
			}
		}
		if !foundSOA {
			t.Fatalf("SOA record not found for data source assertion")
		}
	}

	// 4) Zone metadata: verify resource
	{
		req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/zones/test.example.com./metadata")
		var metadataList []zoneMetadata
		doJSON(t, req, &metadataList)

		metadataByKind := map[string][]string{}
		for _, entry := range metadataList {
			metadataByKind[entry.Kind] = entry.Metadata
		}

		assertMetadataKindValues(t, metadataByKind, "ALSO-NOTIFY", []string{"192.0.2.10", "192.0.2.11:5300"})
		assertMetadataKindValues(t, metadataByKind, "ALLOW-AXFR-FROM", []string{"AUTO-NS", "2001:db8::/48"})
	}

	// 5) Reverse zone: 172.16.0.0/24
	{
		reverseZoneName := ipv4ReverseZoneName(t, "172.16.0.0/24")
		req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/zones/"+reverseZoneName)
		var zone authZone
		doJSON(t, req, &zone)

		if zone.Name != reverseZoneName {
			t.Fatalf("reverse zone name: got %q, want %q", zone.Name, reverseZoneName)
		}
		if zone.Kind != "Master" {
			t.Fatalf("reverse zone kind: got %q, want %q", zone.Kind, "Master")
		}

		// Check PTR record for 172.16.0.10 -> host01.test.example.com.
		ptrName := ipv4PtrName("172.16.0.10")
		var foundPTR bool
		for _, rrset := range zone.RRSets {
			if rrset.Name == ptrName && rrset.Type == "PTR" {
				foundPTR = true
				if rrset.TTL != 30 {
					t.Fatalf("PTR record TTL: got %d, want 30", rrset.TTL)
				}
				if len(rrset.Records) == 0 {
					t.Fatalf("PTR record has no records")
				}
				if rrset.Records[0].Content != "host01.test.example.com." {
					t.Fatalf("PTR record content: got %q, want %q", rrset.Records[0].Content, "host01.test.example.com.")
				}
				break
			}
		}
		if !foundPTR {
			t.Fatalf("PTR record %q not found in reverse zone", ptrName)
		}
	}
}

// Test the Recursor config resources created by Terraform.
func TestPowerDNSRecursorConfig(t *testing.T) {
	base := recursorBaseURL()

	checkCfg := func(name string, want []string) {
		req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/config/"+name)
		var cfg recursorConfig
		doJSON(t, req, &cfg)

		if cfg.Name != name {
			t.Fatalf("config %s: name mismatch: got %q, want %q", name, cfg.Name, name)
		}
		if len(cfg.Value) != len(want) {
			t.Fatalf("config %s: value length mismatch: got %d, want %d", name, len(cfg.Value), len(want))
		}
		gotSet := map[string]bool{}
		for _, v := range cfg.Value {
			gotSet[v] = true
		}
		for _, w := range want {
			if !gotSet[w] {
				t.Fatalf("config %s: missing value %q in %v", name, w, cfg.Value)
			}
		}
	}

	exp := []string{"192.168.0.0/16", "10.0.0.0/8"}
	checkCfg("allow-from", exp)
	checkCfg("allow-notify-from", exp)
}

// Test the Recursor forward zone created by Terraform.
func TestPowerDNSRecursorForwardZone(t *testing.T) {
	base := recursorBaseURL()

	req := newRequest(t, http.MethodGet, base, "/api/v1/servers/localhost/zones/example.com.")
	var zone recursorForwardZone
	doJSON(t, req, &zone)

	if zone.Name != "example.com." {
		t.Fatalf("recursor forward zone name: got %q, want %q", zone.Name, "example.com.")
	}
	if zone.Kind != "Forwarded" {
		t.Fatalf("recursor forward zone kind: got %q, want %q", zone.Kind, "Forwarded")
	}
	if len(zone.Servers) != 1 {
		t.Fatalf("recursor forward zone servers: expected exactly 1 server, got %v", zone.Servers)
	}

	server := zone.Servers[0]

	// Must match IPv4:5300
	ipv4WithPort := regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}:5300$`)

	if !ipv4WithPort.MatchString(server) {
		t.Fatalf("recursor forward zone server: got %q, want <ipv4>:5300", server)
	}
}
