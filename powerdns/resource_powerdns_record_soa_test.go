package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

func TestAccPDNSRecordSOA_Basic(t *testing.T) {
	resourceName := "powerdns_record_soa.test-soa"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordSOADestroy,
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
					resource.TestCheckResourceAttr(resourceName, "disabled", "false"),
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordSOADestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordSOAConfigUpdate1,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "disabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "refresh", "7200"),
				),
			},
			{
				Config: testPDNSRecordSOAConfigUpdate2,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "ttl", "7200"),
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "refresh", "14400"),
				),
			},
		},
	})
}

// With no serial configured, PowerDNS may advance the serial when another SOA
// field changes. The plan must not promise the old serial.
func TestAccPDNSRecordSOA_UpdateOmittedSerial(t *testing.T) {
	resourceName := "powerdns_record_soa.test-soa-omit"
	config := func(ttl, refresh int) string {
		return fmt.Sprintf(`
resource "powerdns_zone" "test-soa-omit" {
	name = "test-soa-omit-sysa.xyz."
	kind = "MASTER"
}

resource "powerdns_record_soa" "test-soa-omit" {
	zone    = powerdns_zone.test-soa-omit.name
	name    = "test-soa-omit-sysa.xyz."
	ttl     = %d
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	refresh = %d
	retry   = 600
	expire  = 1209600
	minimum = 6400
}`, ttl, refresh)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordSOADestroy,
		Steps: []resource.TestStep{
			{
				Config: config(3600, 7200),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordSOAExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "serial"),
				),
			},
			{
				Config: config(7200, 14400),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "ttl", "7200"),
					resource.TestCheckResourceAttr(resourceName, "refresh", "14400"),
					resource.TestCheckResourceAttrSet(resourceName, "serial"),
				),
			},
			{
				Config:   config(7200, 14400),
				PlanOnly: true,
			},
		},
	})
}

func TestAccPDNSRecordSOA_Import(t *testing.T) {
	resourceName := "powerdns_record_soa.test-soa-imp"
	resourceID := `{"zone":"test-soa-imp-sysa.xyz.","id":"test-soa-imp-sysa.xyz.:::SOA"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordSOADestroy,
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
				// A configured serial of 0 stays 0 in state, while an import
				// reads the serial PowerDNS computed.
				ImportStateVerifyIgnore: []string{"serial"},
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
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSRecordSOABlockedConfig,
				ExpectError: regexp.MustCompile(`use the\s+powerdns_record_soa\s+resource\s+instead`),
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
	disabled = true
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

// With no serial configured, an update sends the current serial and accepts
// whatever PowerDNS advances it to.
func TestSOAUpdateSendsCurrentSerialAndTakesServerValue(t *testing.T) {
	const zone = "soa.example."
	content := ""
	ttl := 3600
	var sent []string
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.Method {
		case http.MethodPatch:
			var body struct {
				RRSets []ResourceRecordSet `json:"rrsets"`
			}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			rec := body.RRSets[0].Records[0]
			sent = append(sent, rec.Content)
			ttl = body.RRSets[0].TTL
			// The server advances the serial on every change.
			fields := strings.Fields(rec.Content)
			var serial int
			_, err := fmt.Sscan(fields[2], &serial)
			assert.NoError(t, err)
			fields[2] = fmt.Sprint(serial + 1)
			content = strings.Join(fields, " ")
			return jsonResponse(http.StatusNoContent, ""), nil
		default:
			return jsonResponse(http.StatusOK, fmt.Sprintf(
				`{"name":%q,"rrsets":[{"name":%q,"type":"SOA","ttl":%d,"records":[{"content":%q,"disabled":false}]}]}`,
				zone, zone, ttl, content)), nil
		}
	})
	h := newResourceHarness(t, newRecordSOAResource(), &ProviderClients{PDNS: client})
	config := func(ttl int64) map[string]tftypes.Value {
		return map[string]tftypes.Value{
			"zone": tfString(zone), "name": tfString(zone), "ttl": tfInt(ttl),
			"mname": tfString("ns1.example."), "rname": tfString("host.example."),
			"refresh": tfInt(7200), "retry": tfInt(600), "expire": tfInt(1209600), "minimum": tfInt(6400),
		}
	}

	created, diags := h.create(config(3600))
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, int64(1), tfAttrInt(t, created, "serial"), "creating with no serial sends 0 and the server advances it")

	updated, diags := h.updateWithUnknown(created, config(7200), "serial")
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Contains(t, sent[len(sent)-1], " 1 7200 ", "the update must send the current serial, not 0")
	assert.Equal(t, int64(2), tfAttrInt(t, updated, "serial"), "the new serial comes from the server")
}

func TestSOACreateKeepsStateWhenReadFails(t *testing.T) {
	client := newTestClient(writesSucceedReadsFail(http.StatusNoContent, "", 0, ""))
	assertCreateKeepsState(t, newRecordSOAResource(), &ProviderClients{PDNS: client},
		map[string]tftypes.Value{
			"zone": tfString("kept.example."), "name": tfString("kept.example."), "ttl": tfInt(3600),
			"mname": tfString("ns1.example."), "rname": tfString("host.example."),
			"refresh": tfInt(7200), "retry": tfInt(600), "expire": tfInt(1209600), "minimum": tfInt(6400),
		}, "kept.example.:::SOA")
}

// soaServerFake serves one SOA record and advances its serial on every write
// unless advance is false.
type soaServerFake struct {
	t       *testing.T
	zone    string
	content string
	ttl     int
	advance bool
	sent    []string
}

func (f *soaServerFake) handle(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodPatch {
		var body struct {
			RRSets []ResourceRecordSet `json:"rrsets"`
		}
		assert.NoError(f.t, json.NewDecoder(r.Body).Decode(&body))
		rec := body.RRSets[0].Records[0]
		f.sent = append(f.sent, rec.Content)
		f.ttl = body.RRSets[0].TTL
		fields := strings.Fields(rec.Content)
		if f.advance {
			var serial int
			_, err := fmt.Sscan(fields[2], &serial)
			assert.NoError(f.t, err)
			fields[2] = fmt.Sprint(serial + 1)
		}
		f.content = strings.Join(fields, " ")
		return jsonResponse(http.StatusNoContent, ""), nil
	}
	return jsonResponse(http.StatusOK, fmt.Sprintf(
		`{"name":%q,"rrsets":[{"name":%q,"type":"SOA","ttl":%d,"records":[{"content":%q,"disabled":false}]}]}`,
		f.zone, f.zone, f.ttl, f.content)), nil
}

func soaConfig(zone string, ttl int64, serial *int64) map[string]tftypes.Value {
	config := map[string]tftypes.Value{
		"zone": tfString(zone), "name": tfString(zone), "ttl": tfInt(ttl),
		"mname": tfString("ns1.example."), "rname": tfString("host.example."),
		"refresh": tfInt(7200), "retry": tfInt(600), "expire": tfInt(1209600), "minimum": tfInt(6400),
	}
	if serial != nil {
		config["serial"] = tfInt(*serial)
	}
	return config
}

// server_serial always holds the number PowerDNS reports, and an update never
// sends 0 or an old number when serial is omitted or 0.
func TestSOAServerSerial(t *testing.T) {
	const zone = "soa.example."
	zero, explicit := int64(0), int64(2026112801)
	for _, tc := range []struct {
		name       string
		serial     *int64
		wantCreate string // serial in the created record
	}{
		{"omitted", nil, " 0 7200 "},
		{"zero", &zero, " 0 7200 "},
		{"explicit", &explicit, " 2026112801 7200 "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &soaServerFake{t: t, zone: zone, advance: true}
			h := newResourceHarness(t, newRecordSOAResource(), &ProviderClients{PDNS: newTestClient(fake.handle)})

			created, diags := h.create(soaConfig(zone, 3600, tc.serial))
			assert.False(t, diags.HasError(), "%v", diags)
			assert.Contains(t, fake.sent[0], tc.wantCreate)
			createdServer := tfAttrInt(t, created, "server_serial")
			assert.Equal(t, tfAttrInt(t, created, "serial"), createdServer, "the server value is reported next to serial")
			assert.NotZero(t, createdServer)

			updated, diags := h.updateWithUnknown(created, soaConfig(zone, 7200, tc.serial), "server_serial")
			assert.False(t, diags.HasError(), "%v", diags)
			if tc.serial == nil || *tc.serial == 0 {
				assert.Contains(t, fake.sent[1], fmt.Sprintf(" %d 7200 ", createdServer), "the update must send the serial PowerDNS holds")
			} else {
				assert.Contains(t, fake.sent[1], " 2026112801 7200 ", "an explicit serial is sent as written")
			}
			wantServer := createdServer + 1
			if tc.serial != nil && *tc.serial != 0 {
				wantServer = *tc.serial + 1 // the fake advances whatever it is sent
			}
			assert.Equal(t, wantServer, tfAttrInt(t, updated, "server_serial"), "the new number comes from the server")

			refreshed, diags := h.read(updated)
			assert.False(t, diags.HasError(), "%v", diags)
			assert.Equal(t, tfAttrInt(t, updated, "server_serial"), tfAttrInt(t, refreshed, "server_serial"))
		})
	}
}

// State written by the SDK version has no server_serial. The first refresh
// fills it in and leaves the stored serial alone.
func TestSOAReadFillsServerSerialInSDKState(t *testing.T) {
	const zone = "soa.example."
	fake := &soaServerFake{t: t, zone: zone, ttl: 3600, content: "ns1.example. host.example. 2026112803 7200 600 1209600 6400"}
	h := newResourceHarness(t, newRecordSOAResource(), &ProviderClients{PDNS: newTestClient(fake.handle)})
	sdkState := h.object(map[string]tftypes.Value{
		"id": tfString(zone + ":::SOA"), "zone": tfString(zone), "name": tfString(zone), "ttl": tfInt(3600),
		"disabled": tfBool(false), "mname": tfString("ns1.example."), "rname": tfString("host.example."),
		"serial": tfInt(2026112803), "refresh": tfInt(7200), "retry": tfInt(600), "expire": tfInt(1209600), "minimum": tfInt(6400),
	})
	assert.True(t, tfAttr(t, sdkState, "server_serial").IsNull())

	refreshed, diags := h.read(sdkState)
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, int64(2026112803), tfAttrInt(t, refreshed, "server_serial"))
	assert.Equal(t, int64(2026112803), tfAttrInt(t, refreshed, "serial"))
}

// With serial = 0 the stored serial stays 0 while server_serial follows the
// server, and an explicit serial is kept as written.
func TestAccPDNSRecordSOA_ServerSerial(t *testing.T) {
	config := func(serialLine string, ttl int) string {
		return fmt.Sprintf(`
resource "powerdns_zone" "test-soa-server-serial" {
	name         = "test-soa-server-serial-sysa.xyz."
	kind         = "MASTER"
	soa_edit_api = "DEFAULT"
}

resource "powerdns_record_soa" "test-soa-server-serial" {
	zone    = powerdns_zone.test-soa-server-serial.name
	name    = "test-soa-server-serial-sysa.xyz."
	ttl     = %d
	mname   = "ns1.sysa.xyz."
	rname   = "hostmaster.sysa.xyz."
	%s
	refresh = 7200
	retry   = 600
	expire  = 1209600
	minimum = 6400
}`, ttl, serialLine)
	}
	const name = "powerdns_record_soa.test-soa-server-serial"
	notZero := resource.TestCheckResourceAttrWith(name, "server_serial", func(v string) error {
		if v == "" || v == "0" {
			return fmt.Errorf("server_serial = %q, want the number PowerDNS holds", v)
		}
		return nil
	})
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordSOADestroy,
		Steps: []resource.TestStep{
			{
				Config: config("serial  = 0", 3600),
				Check:  resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr(name, "serial", "0"), notZero),
			},
			{Config: config("serial  = 0", 3600), PlanOnly: true},
			{
				Config: config("serial  = 0", 7200),
				Check:  resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr(name, "serial", "0"), notZero),
			},
			// With serial omitted, a change takes the number from the server.
			{
				Config: config("", 9000),
				Check:  resource.ComposeTestCheckFunc(resource.TestCheckResourceAttrPair(name, "serial", name, "server_serial"), notZero),
			},
			{Config: config("", 9000), PlanOnly: true},
		},
	})
}
