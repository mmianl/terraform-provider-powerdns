package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

func TestShouldPreserveRecordDisabledFlags(t *testing.T) {
	testCases := []struct {
		name               string
		hasExistingID      bool
		disabledConfigured bool
		disabledChanged    bool
		expected           bool
	}{
		{
			name:               "preserve when omitted and unchanged",
			hasExistingID:      true,
			disabledConfigured: false,
			disabledChanged:    false,
			expected:           true,
		},
		{
			name:               "do not preserve when explicitly false",
			hasExistingID:      true,
			disabledConfigured: true,
			disabledChanged:    false,
			expected:           false,
		},
		{
			name:               "do not preserve when disabled changed",
			hasExistingID:      true,
			disabledConfigured: false,
			disabledChanged:    true,
			expected:           false,
		},
		{
			name:               "do not preserve on create",
			hasExistingID:      false,
			disabledConfigured: false,
			disabledChanged:    false,
			expected:           false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldPreserveRecordDisabledFlags(tc.hasExistingID, tc.disabledConfigured, tc.disabledChanged)
			if got != tc.expected {
				t.Fatalf("shouldPreserveRecordDisabledFlags() = %t, want %t", got, tc.expected)
			}
		})
	}
}

func TestConfiguredRRSetDisabledValue(t *testing.T) {
	testCases := []struct {
		name       string
		configured bool
		disabled   bool
		expected   bool
	}{
		{name: "omitted keeps default false", configured: false, disabled: true, expected: false},
		{name: "explicit true is preserved", configured: true, disabled: true, expected: true},
		{name: "explicit false is preserved", configured: true, disabled: false, expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := configuredRRSetDisabledValue(tc.configured, tc.disabled); got != tc.expected {
				t.Fatalf("configuredRRSetDisabledValue() = %t, want %t", got, tc.expected)
			}
		})
	}
}

func TestAccPDNSRecord_Empty(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
	resourceID := `{"zone":"rec-a.sysa.xyz.","id":"test.rec-a.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigA,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "disabled", "false"),
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

func TestAccPDNSRecord_Disabled(t *testing.T) {
	resourceName := "powerdns_record.test-a-disabled"
	resourceID := `{"zone":"rec-adisabled.sysa.xyz.","id":"test.rec-adisabled.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigADisabled,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
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

func TestAccPDNSRecord_DisabledRefreshNoDiff(t *testing.T) {
	resourceName := "powerdns_record.test-a-disabled-refresh"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigADisabledRefresh,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
					testAccCheckPDNSRecordAllDisabled(resourceName, true),
				),
			},
			{
				ResourceName: resourceName,
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
					testAccCheckPDNSRecordAllDisabled(resourceName, true),
				),
			},
		},
	})
}

func TestAccPDNSRecord_OmittedDisabledPreservesExistingFlags(t *testing.T) {
	resourceName := "powerdns_record.test-a-omit-disabled"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigAOmittedDisabledInitial,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "ttl", "60"),
					testAccCheckPDNSRecordAllDisabled(resourceName, true),
				),
			},
			{
				Config: testPDNSRecordConfigAOmittedDisabledUpdated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "ttl", "120"),
					testAccCheckPDNSRecordAllDisabled(resourceName, true),
				),
			},
			{
				ResourceName: resourceName,
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "ttl", "120"),
					testAccCheckPDNSRecordAllDisabled(resourceName, true),
				),
			},
		},
	})
}

// Adding a record to an RRset whose records are all disabled gives a new,
// enabled record, so the combined "disabled" value changes with it.
func TestAccPDNSRecord_AddToDisabledSet(t *testing.T) {
	resourceName := "powerdns_record.test-a-disabled-add"
	config := func(disabledLine string, records string) string {
		return fmt.Sprintf(`
resource "powerdns_zone" "test-zone" {
	name = "rec-a-disabled-add.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-disabled-add" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-a-disabled-add.sysa.xyz."
	type = "A"
	ttl = 60
	%s
	records = %s
}`, disabledLine, records)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: config("disabled = true", `["1.1.1.1"]`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "disabled", "true"),
				),
			},
			{
				Config: config("", `["1.1.1.1", "2.2.2.2"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "disabled", "false"),
				),
			},
			{
				Config:   config("", `["1.1.1.1", "2.2.2.2"]`),
				PlanOnly: true,
			},
		},
	})
}

func TestAccPDNSRecord_WithComments(t *testing.T) {
	resourceName := "powerdns_record.test-a-comments"
	resourceID := `{"zone":"rec-acomments.sysa.xyz.","id":"test.rec-acomments.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckComments(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigAWithComments,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSRecordExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "comments.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "comments.0", "managed-by=terraform"),
					resource.TestCheckResourceAttr(resourceName, "comments.1", "owner=dns-team"),
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
	resourceID := `{"zone":"rec-aptr.sysa.xyz.","id":"test.rec-aptr.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID0 := `{"zone":"rec-count.sysa.xyz.","id":"test-0.rec-count.sysa.xyz.:::A"}`
	resourceID1 := `{"zone":"rec-count.sysa.xyz.","id":"test-1.rec-count.sysa.xyz.:::A"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-aaaa.sysa.xyz.","id":"test.rec-aaaa.sysa.xyz.:::AAAA"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-cname.sysa.xyz.","id":"test.rec-cname.sysa.xyz.:::CNAME"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-hinfo.sysa.xyz.","id":"test.rec-hinfo.sysa.xyz.:::HINFO"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-loc.sysa.xyz.","id":"test.rec-loc.sysa.xyz.:::LOC"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigLOC,
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

func TestAccPDNSRecord_MX(t *testing.T) {
	resourceName := "powerdns_record.test-mx"
	resourceNameMulti := "powerdns_record.test-mx-multi"
	resourceID := `{"zone":"rec-mx.sysa.xyz.","id":"rec-mx.sysa.xyz.:::MX"}`
	resourceIDMulti := `{"zone":"rec-mx.sysa.xyz.","id":"multi.rec-mx.sysa.xyz.:::MX"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-naptr.sysa.xyz.","id":"rec-naptr.sysa.xyz.:::NAPTR"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-ns.sysa.xyz.","id":"lab.rec-ns.sysa.xyz.:::NS"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-spf.sysa.xyz.","id":"rec-spf.sysa.xyz.:::SPF"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-sshfp.sysa.xyz.","id":"ssh.rec-sshfp.sysa.xyz.:::SSHFP"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSRecordConfigSSHFP,
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

func TestAccPDNSRecord_SRV(t *testing.T) {
	resourceName := "powerdns_record.test-srv"
	resourceID := `{"zone":"rec-srv.sysa.xyz.","id":"_redis._tcp.rec-srv.sysa.xyz.:::SRV"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-txt.sysa.xyz.","id":"text.rec-txt.sysa.xyz.:::TXT"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resourceID := `{"zone":"rec-alias.sysa.xyz.","id":"alias.rec-alias.sysa.xyz.:::ALIAS"}`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
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
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSRecordConfigSOA,
				ExpectError: regexp.MustCompile(`use the\s+powerdns_record_soa\s+resource\s+instead`),
			},
		},
	})
}

func testAccCheckPDNSRecordDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_record" {
			continue
		}

		client := testAccProvider.Meta().(*ProviderClients).PDNS
		exists, err := client.RecordExistsByID(context.Background(), rs.Primary.Attributes["zone"], rs.Primary.ID)
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

		client := testAccProvider.Meta().(*ProviderClients).PDNS
		foundRecords, err := client.ListRecordsByID(context.Background(), rs.Primary.Attributes["zone"], rs.Primary.ID)
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

func testAccCheckPDNSRecordAllDisabled(n string, expected bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Record ID is set")
		}

		client := testAccProvider.Meta().(*ProviderClients).PDNS
		foundRecords, err := client.ListRecordsByID(context.Background(), rs.Primary.Attributes["zone"], rs.Primary.ID)
		if err != nil {
			return err
		}
		if len(foundRecords) == 0 {
			return fmt.Errorf("Record does not exist")
		}

		for _, rec := range foundRecords {
			if rec.Disabled != expected {
				return fmt.Errorf("record %q disabled = %t, want %t", rec.Content, rec.Disabled, expected)
			}
		}

		return nil
	}
}

const testPDNSRecordConfigRecordEmpty = `
resource "powerdns_zone" "test-zone" {
	name = "rec-empty.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-empty.sysa.xyz."
	type = "A"
	ttl = 60
	records = [ ]
}`

const testPDNSRecordConfigA = `
resource "powerdns_zone" "test-zone" {
	name = "rec-a.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-a.sysa.xyz."
	type = "A"
	ttl = 60
	records = [ "1.1.1.1", "2.2.2.2" ]
}`

const testPDNSRecordConfigADisabled = `
resource "powerdns_zone" "test-zone" {
	name = "rec-adisabled.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-disabled" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-adisabled.sysa.xyz."
	type = "A"
	ttl = 60
	disabled = true
	records = [ "1.1.1.1", "2.2.2.2" ]
}`

const testPDNSRecordConfigADisabledRefresh = `
resource "powerdns_zone" "test-zone" {
	name = "rec-adisabled-refresh.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-disabled-refresh" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-adisabled-refresh.sysa.xyz."
	type = "A"
	ttl = 60
	disabled = true
	records = [ "1.1.1.1", "2.2.2.2" ]
}`

const testPDNSRecordConfigAOmittedDisabledInitial = `
resource "powerdns_zone" "test-zone" {
	name = "rec-a-omit-disabled.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-omit-disabled" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-a-omit-disabled.sysa.xyz."
	type = "A"
	ttl = 60
	disabled = true
	records = [ "1.1.1.1", "2.2.2.2" ]
}`

const testPDNSRecordConfigAOmittedDisabledUpdated = `
resource "powerdns_zone" "test-zone" {
	name = "rec-a-omit-disabled.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-omit-disabled" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-a-omit-disabled.sysa.xyz."
	type = "A"
	ttl = 120
	records = [ "1.1.1.1", "2.2.2.2" ]
}`

const testPDNSRecordConfigAWithPtr = `
resource "powerdns_zone" "test-zone" {
	name = "rec-aptr.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-ptr" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-aptr.sysa.xyz."
	type = "A"
	ttl = 60
	set_ptr = true
	records = [ "1.1.1.1" ]
}`

const testPDNSRecordConfigAWithComments = `
resource "powerdns_zone" "test-zone" {
	name = "rec-acomments.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-a-comments" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-acomments.sysa.xyz."
	type = "A"
	ttl = 60
	comments = [
		"managed-by=terraform",
		"owner=dns-team",
	]
	records = [ "1.1.1.1" ]
}`

const testPDNSRecordConfigHyphenedWithCount = `
resource "powerdns_zone" "test-zone" {
	name = "rec-count.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-counted" {
	count = "2"
	zone = powerdns_zone.test-zone.name
	name = "test-${count.index}.rec-count.sysa.xyz."
	type = "A"
	ttl = 60
	records = [ "1.1.1.${count.index}" ]
}`

const testPDNSRecordConfigAAAA = `
resource "powerdns_zone" "test-zone" {
	name = "rec-aaaa.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-aaaa" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-aaaa.sysa.xyz."
	type = "AAAA"
	ttl = 60
	records = [ "2001:db8:2000:bf0::1", "2001:db8:2000:bf1::1" ]
}`

const testPDNSRecordConfigCNAME = `
resource "powerdns_zone" "test-zone" {
	name = "rec-cname.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-cname" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-cname.sysa.xyz."
	type = "CNAME"
	ttl = 60
	records = [ "redis.example.com." ]
}`

const testPDNSRecordConfigHINFO = `
resource "powerdns_zone" "test-zone" {
	name = "rec-hinfo.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-hinfo" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-hinfo.sysa.xyz."
	type = "HINFO"
	ttl = 60
	records = [ "\"PC-Intel-2.4ghz\" \"Linux\"" ]
}`

const testPDNSRecordConfigLOC = `
resource "powerdns_zone" "test-zone" {
	name = "rec-loc.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-loc" {
	zone = powerdns_zone.test-zone.name
	name = "test.rec-loc.sysa.xyz."
	type = "LOC"
	ttl = 60
	records = [ "51 56 0.123 N 5 54 0.000 E 4.00m 1.00m 10000.00m 10.00m" ]
}`

const testPDNSRecordConfigMX = `
resource "powerdns_zone" "test-zone" {
	name = "rec-mx.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-mx" {
	zone = powerdns_zone.test-zone.name
	name = "rec-mx.sysa.xyz."
	type = "MX"
	ttl = 60
	records = [ "10 mail.example.com." ]
}`

const testPDNSRecordConfigMXMulti = `
resource "powerdns_zone" "test-zone" {
	name = "rec-mx.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-mx-multi" {
	zone = powerdns_zone.test-zone.name
	name = "multi.rec-mx.sysa.xyz."
	type = "MX"
	ttl = 60
	records = [ "10 mail1.example.com.", "20 mail2.example.com." ]
}`

const testPDNSRecordConfigNAPTR = `
resource "powerdns_zone" "test-zone" {
	name = "rec-naptr.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-naptr" {
	zone = powerdns_zone.test-zone.name
	name = "rec-naptr.sysa.xyz."
	type = "NAPTR"
	ttl = 60
	records = [ "100 50 \"s\" \"z3950+I2L+I2C\" \"\" _z3950._tcp.gatech.edu." ]
}`

const testPDNSRecordConfigNS = `
resource "powerdns_zone" "test-zone" {
	name = "rec-ns.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-ns" {
	zone = powerdns_zone.test-zone.name
	name = "lab.rec-ns.sysa.xyz."
	type = "NS"
	ttl = 60
	records = [ "ns1.rec-ns.sysa.xyz.", "ns2.rec-ns.sysa.xyz." ]
}`

const testPDNSRecordConfigSPF = `
resource "powerdns_zone" "test-zone" {
	name = "rec-spf.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-spf" {
	zone = powerdns_zone.test-zone.name
	name = "rec-spf.sysa.xyz."
	type = "SPF"
	ttl = 60
	records = [ "\"v=spf1 +all\"" ]
}`

const testPDNSRecordConfigSSHFP = `
resource "powerdns_zone" "test-zone" {
	name = "rec-sshfp.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-sshfp" {
	zone = powerdns_zone.test-zone.name
	name = "ssh.rec-sshfp.sysa.xyz."
	type = "SSHFP"
	ttl = 60
	records = [ "1 1 123456789abcdef67890123456789abcdef67890" ]
}`

const testPDNSRecordConfigSRV = `
resource "powerdns_zone" "test-zone" {
	name = "rec-srv.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-srv" {
	zone = powerdns_zone.test-zone.name
	name = "_redis._tcp.rec-srv.sysa.xyz."
	type = "SRV"
	ttl = 60
	records = [ "0 10 6379 redis1.rec-srv.sysa.xyz.", "0 10 6379 redis2.rec-srv.sysa.xyz.", "10 10 6379 redis-replica.rec-srv.sysa.xyz." ]
}`

const testPDNSRecordConfigTXT = `
resource "powerdns_zone" "test-zone" {
	name = "rec-txt.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-txt" {
	zone = powerdns_zone.test-zone.name
	name = "text.rec-txt.sysa.xyz."
	type = "TXT"
	ttl = 60
	records = [ "\"text record payload\"" ]
}`

const testPDNSRecordConfigALIAS = `
resource "powerdns_zone" "test-zone" {
	name = "rec-alias.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "test-alias" {
	zone = powerdns_zone.test-zone.name
	name = "alias.rec-alias.sysa.xyz."
	type = "ALIAS"
	ttl = 3600
	records = [ "www.some-alias.com." ]
}`

const testPDNSRecordConfigSOA = `
resource "powerdns_record" "test-soa" {
	zone = "test-soa-sysa.xyz."
	name = "test-soa-sysa.xyz."
	type = "SOA"
	ttl = 3600
	records = [ "something.something. hostmaster.sysa.xyz. 2019090301 10800 3600 604800 3600" ]
}`

// Two resources naming the same record set would otherwise both report success
// while the server keeps only one of them, leaving a plan that never converges.
func TestAccPDNSRecordDuplicateRRSetIsRejected(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSRecordDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSRecordConfigDuplicateRRSet,
				ExpectError: regexp.MustCompile(`already exists in zone`),
			},
		},
	})
}

const testPDNSRecordConfigDuplicateRRSet = `
resource "powerdns_zone" "dup-guard" {
	name = "dup-guard.sysa.xyz."
	kind = "Native"
}

resource "powerdns_record" "dup-a" {
	zone    = powerdns_zone.dup-guard.name
	name    = "dup.dup-guard.sysa.xyz."
	type    = "A"
	ttl     = 300
	records = ["192.0.2.1"]
}

resource "powerdns_record" "dup-b" {
	zone       = powerdns_zone.dup-guard.name
	name       = "dup.dup-guard.sysa.xyz."
	type       = "A"
	ttl        = 300
	records    = ["192.0.2.2"]
	depends_on = [powerdns_record.dup-a]
}`

// The combined "disabled" value follows the record contents: it keeps its
// stored value while they stay the same and is left to the server otherwise.
func TestDisabledFollowsRecordContents(t *testing.T) {
	h := newResourceHarness(t, newRecordResource(), &ProviderClients{})
	build := func(records ...string) tfsdk.Plan {
		return tfsdk.Plan{Schema: h.schema, Raw: h.object(map[string]tftypes.Value{"records": tfStringSet(records...)})}
	}
	run := func(planned, stored tfsdk.Plan) types.Bool {
		resp := &planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
		keepUnlessRecordsChange{}.PlanModifyBool(context.Background(), planmodifier.BoolRequest{
			ConfigValue: types.BoolNull(), PlanValue: types.BoolUnknown(), StateValue: types.BoolValue(true),
			Plan: planned, State: tfsdk.State{Schema: h.schema, Raw: stored.Raw},
		}, resp)
		assert.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
		return resp.PlanValue
	}
	assert.Equal(t, types.BoolValue(true), run(build("1.1.1.1"), build("1.1.1.1")))
	assert.True(t, run(build("1.1.1.1", "2.2.2.2"), build("1.1.1.1")).IsUnknown())

	// A configured value is never overridden.
	resp := &planmodifier.BoolResponse{PlanValue: types.BoolValue(false)}
	keepUnlessRecordsChange{}.PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		ConfigValue: types.BoolValue(false), PlanValue: types.BoolValue(false), StateValue: types.BoolValue(true),
		Plan: build("1.1.1.1"), State: tfsdk.State{Schema: h.schema, Raw: build("1.1.1.1").Raw},
	}, resp)
	assert.Equal(t, types.BoolValue(false), resp.PlanValue)
}

// A record that PowerDNS accepted but cannot be read back is kept as well.
func TestRecordCreateKeepsStateWhenReadFails(t *testing.T) {
	// The overwrite guard reads the zone first and finds it empty; every
	// later read fails.
	reads := 0
	failing := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch {
			return jsonResponse(http.StatusNoContent, ""), nil
		}
		reads++
		if reads == 1 {
			return jsonResponse(http.StatusOK, `{"name":"kept.example.","rrsets":[]}`), nil
		}
		return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
	})
	h := newResourceHarness(t, newRecordResource(), &ProviderClients{PDNS: failing})
	state, diags := h.create(map[string]tftypes.Value{
		"zone": tfString("kept.example."), "name": tfString("www.kept.example."), "type": tfString("A"),
		"ttl": tfInt(60), "records": tfStringSet("192.0.2.1"),
	})
	assert.True(t, diags.HasError())
	if assert.False(t, state.IsNull(), "the created record must be saved") {
		assert.Equal(t, "www.kept.example.:::A", tfAttrString(t, state, "id"))
		assert.Equal(t, "kept.example.", tfAttrString(t, state, "zone"))
	}
}

// With "disabled" not configured, adding a record keeps the flags of the
// records that exist and enables only the new one. The planned value of
// "disabled" is unknown here, because the combined value changes with it.
func TestRecordUpdatePreservesDisabledFlagsWhenNotConfigured(t *testing.T) {
	const zone = "flags.example."
	const name = "www.flags.example."
	stored := map[string]bool{}
	var order []string
	var lastPatch map[string]bool
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch {
			var body struct {
				RRSets []ResourceRecordSet `json:"rrsets"`
			}
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			stored = map[string]bool{}
			order = nil
			for _, rec := range body.RRSets[0].Records {
				stored[rec.Content] = rec.Disabled
				order = append(order, rec.Content)
			}
			lastPatch = map[string]bool{}
			for k, v := range stored {
				lastPatch[k] = v
			}
			return jsonResponse(http.StatusNoContent, ""), nil
		}
		if len(order) == 0 {
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"name":%q,"rrsets":[]}`, zone)), nil
		}
		records := make([]string, 0, len(order))
		for _, content := range order {
			records = append(records, fmt.Sprintf(`{"content":%q,"disabled":%t}`, content, stored[content]))
		}
		return jsonResponse(http.StatusOK, fmt.Sprintf(
			`{"name":%q,"rrsets":[{"name":%q,"type":"A","ttl":60,"records":[%s]}]}`,
			zone, name, strings.Join(records, ","))), nil
	})
	h := newResourceHarness(t, newRecordResource(), &ProviderClients{PDNS: client})
	config := func(disabled *bool, records ...string) map[string]tftypes.Value {
		values := map[string]tftypes.Value{
			"zone": tfString(zone), "name": tfString(name), "type": tfString("A"),
			"ttl": tfInt(60), "records": tfStringSet(records...),
		}
		if disabled != nil {
			values["disabled"] = tfBool(*disabled)
		}
		return values
	}

	on := true
	created, diags := h.create(config(&on, "192.0.2.1", "192.0.2.2"))
	assert.False(t, diags.HasError(), "%v", diags)
	disabled, _ := tfAttrBool(t, created, "disabled")
	assert.True(t, disabled)

	updated, diags := h.updateWithUnknown(created, config(nil, "192.0.2.1", "192.0.2.2", "192.0.2.3"), "disabled")
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, map[string]bool{"192.0.2.1": true, "192.0.2.2": true, "192.0.2.3": false}, lastPatch,
		"existing flags must be kept and only the new record enabled")
	disabled, _ = tfAttrBool(t, updated, "disabled")
	assert.False(t, disabled, "the combined value follows the server")
}
