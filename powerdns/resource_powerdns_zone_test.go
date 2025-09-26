package powerdns

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestAccPDNSZoneNative(t *testing.T) {
	resourceName := "powerdns_zone.test-native"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigNative,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Native"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneNativeMixedCaps(t *testing.T) {
	resourceName := "powerdns_zone.test-native"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				// using mixed caps config to create resource with test-native name
				Config: testPDNSZoneConfigNativeMixedCaps,
			},
			{
				// using test-native config with Native to confirm plan doesn't generate diff
				ResourceName:       resourceName,
				Config:             testPDNSZoneConfigNative,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
			},
		},
	})
}

func TestAccPDNSZoneNativeSmallCaps(t *testing.T) {
	resourceName := "powerdns_zone.test-native"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				// using small caps config to create resource with test-native name
				Config: testPDNSZoneConfigNativeSmallCaps,
			},
			{
				// using test-native config with Native to confirm plan doesn't generate diff
				ResourceName:       resourceName,
				Config:             testPDNSZoneConfigNative,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
			},
		},
	})
}

func TestAccPDNSZoneMaster(t *testing.T) {
	resourceName := "powerdns_zone.test-master"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMaster,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneMasterSOAAPIEDIT(t *testing.T) {
	resourceName := "powerdns_zone.test-master-soa-edit-api"
	resourceSOAEDITAPI := `DEFAULT`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMasterSOAEDITAPI,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master-soa-edit-api.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "soa_edit_api", resourceSOAEDITAPI),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneMasterSOAAPIEDITEmpty(t *testing.T) {
	resourceName := "powerdns_zone.test-master-soa-edit-api-empty"
	resourceSOAEDITAPI := `""`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMasterSOAEDITAPIEmpty,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master-soa-edit-api-empty.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "soa_edit_api", resourceSOAEDITAPI),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneMasterSOAAPIEDITUndefined(t *testing.T) {
	resourceName := "powerdns_zone.test-master-soa-edit-api-undefined"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigMasterSOAEDITAPIUndefined,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "master-soa-edit-api-undefined.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneAccount(t *testing.T) {
	resourceName := "powerdns_zone.test-account"
	resourceAccount := `test`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigAccount,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "account.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "account", resourceAccount),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneAccountEmpty(t *testing.T) {
	resourceName := "powerdns_zone.test-account-empty"
	resourceAccount := ``

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigAccountEmpty,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "account-empty.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "account", resourceAccount),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneAccountUndefined(t *testing.T) {
	resourceName := "powerdns_zone.test-account-undefined"
	resourceAccount := `admin`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigAccountUndefined,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "account-undefined.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Master"),
					resource.TestCheckResourceAttr(resourceName, "account", resourceAccount),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneSlave(t *testing.T) {
	resourceName := "powerdns_zone.test-slave"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlave,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "slave.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Slave"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithMasters(t *testing.T) {
	resourceName := "powerdns_zone.test-slave-with-masters"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		//CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlaveWithMasters,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "slave-with-masters.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Slave"),
					resource.TestCheckResourceAttr(resourceName, "masters.1048647934", "2.2.2.2"),
					resource.TestCheckResourceAttr(resourceName, "masters.251826590", "1.1.1.1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithMastersWithPort(t *testing.T) {
	resourceName := "powerdns_zone.test-slave-with-masters-with-port"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		//CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneConfigSlaveWithMastersWithPort,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSZoneExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "slave-with-masters-with-port.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "Slave"),
					resource.TestCheckResourceAttr(resourceName, "masters.1048647934", "2.2.2.2"),
					resource.TestCheckResourceAttr(resourceName, "masters.1686215786", "1.1.1.1:1111"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPDNSZoneSlaveWithMastersWithInvalidPort(t *testing.T) {

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigSlaveWithMastersWithInvalidPort,
				ExpectError: regexp.MustCompile("Invalid port value in masters atribute"),
			},
		},
	})
}
func TestAccPDNSZoneSlaveWithInvalidMasters(t *testing.T) {

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigSlaveWithInvalidMasters,
				ExpectError: regexp.MustCompile("values in masters list attribute must be valid IPs"),
			},
		},
	})
}

func TestAccPDNSZoneMasterWithMasters(t *testing.T) {

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneConfigMasterWithMasters,
				ExpectError: regexp.MustCompile("masters attribute is supported only for Slave kind"),
			},
		},
	})
}

func testAccCheckPDNSZoneDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_zone" {
			continue
		}

		client := testAccProvider.Meta().(*Client)
		exists, err := client.ZoneExists(rs.Primary.Attributes["zone"])
		if err != nil {
			return fmt.Errorf("Error checking if zone still exists: %#v", rs.Primary.ID)
		}
		if exists {
			return fmt.Errorf("Zone still exists: %#v", rs.Primary.ID)
		}

	}
	return nil
}

func testAccCheckPDNSZoneExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		client := testAccProvider.Meta().(*Client)
		exists, err := client.ZoneExists(rs.Primary.Attributes["name"])
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("Zone does not exist: %#v", rs.Primary.ID)
		}
		return nil
	}
}

func TestResourcePDNSZoneCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones" && r.Method == "POST" {
			var zone ZoneInfo
			err := json.NewDecoder(r.Body).Decode(&zone)
			assert.NoError(t, err)
			zone.ID = zone.Name
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				ID:   "example.com.",
				Name: "example.com.",
				Kind: "Native",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
		"name":        "example.com.",
		"kind":        "Native",
		"nameservers": []interface{}{"ns1.example.com.", "ns2.example.com."},
	})

	err := resourcePDNSZoneCreate(rd, client)
	if err != nil {
		t.Fatalf("resourcePDNSZoneCreate failed: %v", err)
	}

	if rd.Id() != "example.com." {
		t.Errorf("Expected ID 'example.com.', got '%s'", rd.Id())
	}
}

const testPDNSZoneConfigNative = `
resource "powerdns_zone" "test-native" {
	name = "sysa.abc."
	kind = "Native"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
}`

const testPDNSZoneConfigNativeMixedCaps = `
resource "powerdns_zone" "test-native" {
	name = "sysa.abc."
	kind = "NaTIve"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
}`

const testPDNSZoneConfigNativeSmallCaps = `
resource "powerdns_zone" "test-native" {
	name = "sysa.abc."
	kind = "native"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
}`

const testPDNSZoneConfigMaster = `
resource "powerdns_zone" "test-master" {
	name = "master.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
}`

const testPDNSZoneConfigMasterSOAEDITAPI = `
resource "powerdns_zone" "test-master-soa-edit-api" {
	name = "master-soa-edit-api.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
	soa_edit_api = "DEFAULT"
}`

const testPDNSZoneConfigMasterSOAEDITAPIEmpty = `
resource "powerdns_zone" "test-master-soa-edit-api-empty" {
	name = "master-soa-edit-api-empty.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
	soa_edit_api = "\"\""
}`

const testPDNSZoneConfigMasterSOAEDITAPIUndefined = `
resource "powerdns_zone" "test-master-soa-edit-api-undefined" {
	name = "master-soa-edit-api-undefined.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
}`

const testPDNSZoneConfigAccount = `
resource "powerdns_zone" "test-account" {
	name = "account.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
	account = "test"
}`

const testPDNSZoneConfigAccountEmpty = `
resource "powerdns_zone" "test-account-empty" {
	name = "account-empty.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
	account = ""
}`

const testPDNSZoneConfigAccountUndefined = `
resource "powerdns_zone" "test-account-undefined" {
	name = "account-undefined.sysa.abc."
	kind = "Master"
	nameservers = ["ns1.sysa.abc.", "ns2.sysa.abc."]
	soa_edit_api = "DEFAULT"
}`

const testPDNSZoneConfigSlave = `
resource "powerdns_zone" "test-slave" {
	name = "slave.sysa.abc."
	kind = "Slave"
	nameservers = []
}`

const testPDNSZoneConfigSlaveWithMasters = `
resource "powerdns_zone" "test-slave-with-masters" {
	name = "slave-with-masters.sysa.abc."
	kind = "Slave"
	masters = ["1.1.1.1", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithMastersWithPort = `
resource "powerdns_zone" "test-slave-with-masters-with-port" {
	name = "slave-with-masters-with-port.sysa.abc."
	kind = "Slave"
	masters = ["1.1.1.1:1111", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithMastersWithInvalidPort = `
resource "powerdns_zone" "test-slave-with-masters-with-invalid-port" {
	name = "slave-with-masters-with-invalid-port.sysa.abc."
	kind = "Slave"
	masters = ["1.1.1.1:111111", "2.2.2.2"]
}`

const testPDNSZoneConfigSlaveWithInvalidMasters = `
resource "powerdns_zone" "test-slave-with-invalid-masters" {
	name = "slave-with-invalid-masters.sysa.abc."
	kind = "Slave"
	masters = ["example.com", "2.2.2.2"]
}`

const testPDNSZoneConfigMasterWithMasters = `
resource "powerdns_zone" "test-master-with-masters" {
	name = "master-with-masters.sysa.abc."
	kind = "Master"
	masters = ["1.1.1.1", "2.2.2.2"]
}`

// Additional test functions for resource operations
func TestResourcePDNSZoneUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "PUT" {
			w.WriteHeader(204)
		} else if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." {
			zone := ZoneInfo{
				ID:         "example.com.",
				Name:       "example.com.",
				Kind:       "Master",
				Account:    "test",
				SoaEditAPI: "DEFAULT",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
		"name":         "example.com.",
		"kind":         "Master",
		"account":      "admin",
		"soa_edit_api": "DEFAULT",
	})
	rd.SetId("example.com.")

	err := resourcePDNSZoneUpdate(rd, client)
	if err != nil {
		t.Fatalf("resourcePDNSZoneUpdate failed: %v", err)
	}
}

func TestResourcePDNSZoneDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones/example.com." && r.Method == "DELETE" {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
		"name": "example.com.",
		"kind": "Native",
	})
	rd.SetId("example.com.")

	err := resourcePDNSZoneDelete(rd, client)
	if err != nil {
		t.Fatalf("resourcePDNSZoneDelete failed: %v", err)
	}
}

func TestResourcePDNSZoneExists(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		expected       bool
		expectError    bool
	}{
		{
			name:           "Zone exists",
			serverResponse: 200,
			expected:       true,
			expectError:    false,
		},
		{
			name:           "Zone does not exist",
			serverResponse: 404,
			expected:       false,
			expectError:    false,
		},
		{
			name:           "Server error",
			serverResponse: 500,
			expected:       false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/servers":
					w.WriteHeader(200)
				case "/api/v1/servers/localhost":
					serverInfo := map[string]interface{}{
						"version": "4.5.0",
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(200)
					if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
						t.Errorf("Failed to encode server info response: %v", err)
					}
				case "/api/v1/servers/localhost/zones/example.com.":
					if tt.serverResponse == 500 {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(500)
						if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Internal Server Error"}); err != nil {
							t.Errorf("Failed to encode error response: %v", err)
						}
					} else {
						w.WriteHeader(tt.serverResponse)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

			rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
				"name": "example.com.",
				"kind": "Native",
			})
			rd.SetId("example.com.")

			exists, err := resourcePDNSZoneExists(rd, client)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if exists != tt.expected {
					t.Errorf("Expected exists=%v, got %v", tt.expected, exists)
				}
			}
		})
	}
}

func TestResourcePDNSZoneCreateSlave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones" && r.Method == "POST" {
			var zone ZoneInfo
			if err := json.NewDecoder(r.Body).Decode(&zone); err != nil {
				t.Errorf("Failed to decode zone request: %v", err)
			}
			if zone.Kind == "Slave" && len(zone.Masters) > 0 {
				zone.ID = zone.Name
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				if err := json.NewEncoder(w).Encode(zone); err != nil {
					t.Errorf("Failed to encode zone response: %v", err)
				}
			} else {
				w.WriteHeader(400)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones/slave.example.com." {
			zone := ZoneInfo{
				ID:      "slave.example.com.",
				Name:    "slave.example.com.",
				Kind:    "Slave",
				Masters: []string{"1.2.3.4", "5.6.7.8"},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(zone); err != nil {
				t.Errorf("Failed to encode zone response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
		"name":    "slave.example.com.",
		"kind":    "Slave",
		"masters": []interface{}{"1.2.3.4", "5.6.7.8"},
	})

	err := resourcePDNSZoneCreate(rd, client)
	if err != nil {
		t.Fatalf("resourcePDNSZoneCreate failed: %v", err)
	}

	if rd.Id() != "slave.example.com." {
		t.Errorf("Expected ID 'slave.example.com.', got '%s'", rd.Id())
	}
}

func TestResourcePDNSZoneCreateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/servers" {
			w.WriteHeader(200)
		} else if r.URL.Path == "/api/v1/servers/localhost" {
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/servers/localhost/zones" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Zone creation failed"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
		"name": "error.example.com.",
		"kind": "Native",
	})

	err := resourcePDNSZoneCreate(rd, client)
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}

func TestResourcePDNSZoneReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/servers":
			w.WriteHeader(200)
		case "/api/v1/servers/localhost":
			serverInfo := map[string]interface{}{
				"version": "4.5.0",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(serverInfo); err != nil {
				t.Errorf("Failed to encode server info response: %v", err)
			}
		case "/api/v1/servers/localhost/zones/error.example.com.":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			if err := json.NewEncoder(w).Encode(errorResponse{ErrorMsg: "Zone not found"}); err != nil {
				t.Errorf("Failed to encode error response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "", "test", nil, false, "10", 300)

	rd := schema.TestResourceDataRaw(t, resourcePDNSZone().Schema, map[string]interface{}{
		"name": "error.example.com.",
		"kind": "Native",
	})
	rd.SetId("error.example.com.")

	err := resourcePDNSZoneRead(rd, client)
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}
