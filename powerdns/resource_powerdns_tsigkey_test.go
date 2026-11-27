package powerdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestTSIGKeyUpdateSendsSecretOnlyForRotation(t *testing.T) {
	tests := []struct {
		name           string
		oldVersion     int
		newVersion     int
		storedRotation bool
		wantKey        bool
	}{
		{name: "unchanged write-only version", oldVersion: 1, newVersion: 1},
		{name: "write-only rotation", oldVersion: 1, newVersion: 2, wantKey: true},
		{name: "unchanged stored secret"},
		{name: "stored secret rotation", storedRotation: true, wantKey: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := testTSIGSecret
			puts := 0
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/v1/servers/localhost/tsigkeys" {
					return jsonResponse(http.StatusOK, `[{"id":"tf-old.","algorithm":"hmac-sha256"}]`), nil
				}
				if r.URL.Query().Get("algorithm") != "hmac-sha256" {
					t.Error("managed reads and updates must select the algorithm in state")
				}
				if r.Method == http.MethodPut {
					puts++
					var payload map[string]string
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					key, hasKey := payload["key"]
					if hasKey != tt.wantKey {
						t.Errorf("secret present in update: got %t, want %t", hasKey, tt.wantKey)
					}
					if hasKey {
						if key != testTSIGSecretRotated {
							t.Error("rotation did not send the requested secret")
						}
						secret = key
					}
					if payload["name"] != "tf-old" {
						t.Error("update did not send the requested name")
					}
				}
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":%q,"name":%q,"algorithm":"hmac-sha256","key":%q}`, "tf-old.", "tf-old", secret)), nil
			})
			h := newResourceHarness(t, newTSIGKeyResource(), &ProviderClients{PDNS: client})

			prior := map[string]tftypes.Value{
				"id": tfString("tf-old."), "name": tfString("tf-old"), "algorithm": tfString("hmac-sha256"),
			}
			config := map[string]tftypes.Value{"name": tfString("tf-old"), "algorithm": tfString("hmac-sha256")}
			if tt.oldVersion != 0 {
				prior["key_wo_version"] = tfInt(int64(tt.oldVersion))
				// Deliberately change the write-only input even when its version
				// is unchanged, to ensure that does not request a rotation.
				config["key_wo"] = tfString(testTSIGSecretRotated)
				config["key_wo_version"] = tfInt(int64(tt.newVersion))
			} else {
				prior["key"] = tfString(testTSIGSecret)
				config["key"] = tfString(testTSIGSecret)
				if tt.storedRotation {
					config["key"] = tfString(testTSIGSecretRotated)
				}
			}
			state := h.object(prior)
			updated, diags := h.update(state, config)
			if diags.HasError() {
				t.Fatalf("update failed: %v", diags)
			}
			wantPuts := 0
			if tt.wantKey {
				wantPuts = 1
			}
			if id := tfAttrString(t, updated, "id"); puts != wantPuts || id != "tf-old." {
				t.Fatalf("update calls=%d id=%q, want %d updates and id %q", puts, id, wantPuts, "tf-old.")
			}
		})
	}
}

// A trailing dot on the name is optional, so a refresh keeps whichever spelling
// the configuration uses, while a different name is a genuinely different key.
func TestTSIGKeyNameSemanticEquality(t *testing.T) {
	ctx := context.Background()
	prior := newSemanticString(tsigNameType, "transfer-key")
	for name, want := range map[string]bool{"transfer-key": true, "transfer-key.": true, "renamed-key": false} {
		equal, diags := newSemanticString(tsigNameType, name).StringSemanticEquals(ctx, prior)
		if diags.HasError() {
			t.Fatal(diags)
		}
		if equal != want {
			t.Fatalf("name %q equal to transfer-key: got %t, want %t", name, equal, want)
		}
	}
}

func TestTSIGKeyDeletePreservesOtherAlgorithms(t *testing.T) {
	for _, managedPresent := range []bool{true, false} {
		t.Run(fmt.Sprintf("managedPresent=%t", managedPresent), func(t *testing.T) {
			keys := map[string]string{"hmac-sha384": testTSIGSecretRotated}
			if managedPresent {
				keys["hmac-sha256"] = testTSIGSecret
			}
			deletes := 0
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v1/servers/localhost/tsigkeys" {
					var listed []TSIGKey
					for algorithm := range keys {
						listed = append(listed, TSIGKey{ID: "shared-key.", Name: "shared-key", Algorithm: algorithm})
					}
					body, err := json.Marshal(listed)
					if err != nil {
						t.Fatal(err)
					}
					return jsonResponse(http.StatusOK, string(body)), nil
				}
				if r.URL.Path != "/api/v1/servers/localhost/tsigkeys/shared-key." {
					t.Fatalf("unexpected TSIG endpoint: %s", r.URL.Path)
				}
				algorithm := r.URL.Query().Get("algorithm")
				if r.Method == http.MethodGet {
					return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
				}
				if r.Method != http.MethodDelete {
					t.Fatalf("unexpected TSIG operation: %s", r.Method)
				}
				deletes++
				// Model the API's destructive fallback when no filter is supplied.
				if algorithm == "" {
					clear(keys)
				} else {
					delete(keys, algorithm)
				}
				return jsonResponse(http.StatusNoContent, ``), nil
			})
			// An imported resource identifies its managed algorithm through state.
			h := newResourceHarness(t, newTSIGKeyResource(), &ProviderClients{PDNS: client})
			state := h.object(map[string]tftypes.Value{
				"id": tfString("shared-key."), "name": tfString("shared-key"),
				"algorithm": tfString("hmac-sha256"), "key": tfString(testTSIGSecret),
			})
			if diags := h.destroy(state); diags.HasError() {
				t.Fatalf("destroy failed: %v", diags)
			}
			if _, exists := keys["hmac-sha256"]; exists {
				t.Fatal("destroy did not remove the managed algorithm")
			}
			if keys["hmac-sha384"] != testTSIGSecretRotated {
				t.Fatal("destroy changed the unrelated algorithm or its secret")
			}
			if (deletes == 1) != managedPresent || deletes > 1 {
				t.Fatalf("delete requests=%d, managed algorithm present=%t", deletes, managedPresent)
			}
		})
	}
}

func TestTSIGKeyReadWriteOnlyClearsPersistedSecret(t *testing.T) {
	for _, returnedSecret := range []string{testTSIGSecret, ""} {
		name := "API returns secret"
		if returnedSecret == "" {
			name = "API omits secret"
		}
		t.Run(name, func(t *testing.T) {
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":"tf-transition.","name":"tf-transition","algorithm":"hmac-sha256","key":%q}`, returnedSecret)), nil
			})
			h := newResourceHarness(t, newTSIGKeyResource(), &ProviderClients{PDNS: client})
			// Model a generated or explicit secret retained in prior state.
			state := h.object(map[string]tftypes.Value{
				"id": tfString("tf-transition."), "name": tfString("tf-transition"),
				"algorithm": tfString("hmac-sha256"), "key_wo_version": tfInt(1),
				"key": tfString(testTSIGSecret),
			})
			for i := 0; i < 2; i++ {
				refreshed, diags := h.read(state)
				if diags.HasError() {
					t.Fatalf("refresh failed: %v", diags)
				}
				if !tfAttr(t, refreshed, "key").IsNull() || !tfAttr(t, refreshed, "key_wo").IsNull() {
					t.Fatal("write-only mode retained secret material in state")
				}
				state = refreshed
			}
		})
	}
}

func TestAccPDNSTSIGKeySwitchToWriteOnly(t *testing.T) {
	for _, initialMode := range []string{"generated", "explicit"} {
		t.Run(initialMode, func(t *testing.T) {
			const resourceName = "powerdns_tsigkey.test-transition"
			name := "tf-wo-transition-" + initialMode
			initialKey := ""
			if initialMode == "explicit" {
				initialKey = fmt.Sprintf("\n  key = %q", testTSIGSecret)
			}
			initial := fmt.Sprintf(`
resource "powerdns_tsigkey" "test-transition" {
  name      = %q
  algorithm = "hmac-sha256"%s
}`, name, initialKey)
			// Reuse the explicit secret to cover clearing a still-active value.
			writeOnly := fmt.Sprintf(`
resource "powerdns_tsigkey" "test-transition" {
  name           = %q
  algorithm      = "hmac-sha256"
  key_wo         = %q
  key_wo_version = 1
}`, name, testTSIGSecret)
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
				Steps: []resource.TestStep{
					{
						Config: initial,
						Check:  resource.TestCheckResourceAttrSet(resourceName, "key"),
					},
					{
						Config: writeOnly,
						Check: resource.ComposeTestCheckFunc(
							testAccCheckPDNSTSIGKeyNoPersistedSecret(resourceName),
							resource.TestCheckNoResourceAttr(resourceName, "key_wo"),
							resource.TestCheckResourceAttr(resourceName, "id", name+"."),
							testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
						),
					},
					{Config: writeOnly, PlanOnly: true},
				},
			})
		})
	}
}

func TestAccPDNSTSIGKeyGenerated(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-generated"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigGenerated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "tf-generated"),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "hmac-sha256"),
					// The server generates the secret when none is supplied.
					resource.TestCheckResourceAttrSet(resourceName, "key"),
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

func TestAccPDNSTSIGKeyWriteOnly(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-wo"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigWriteOnly(testTSIGSecret, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					// The secret must never reach state.
					resource.TestCheckNoResourceAttr(resourceName, "key_wo"),
					testAccCheckPDNSTSIGKeyNoPersistedSecret(resourceName),
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
				),
			},
			{
				// A new value without a new version is not noticed.
				Config:   testPDNSTSIGKeyConfigWriteOnly(testTSIGSecretRotated, 1),
				PlanOnly: true,
			},
			{
				Config: testPDNSTSIGKeyConfigWriteOnly(testTSIGSecretRotated, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecretRotated),
					testAccCheckPDNSTSIGKeyNoPersistedSecret(resourceName),
				),
			},
		},
	})
}

func TestAccPDNSTSIGKeyWriteOnlyRenameReplacesKey(t *testing.T) {
	const resourceName = "powerdns_tsigkey.test-wo"
	initial := testPDNSTSIGKeyConfigWriteOnlyNamed("tf-wo-rename", testTSIGSecret, 1)
	renamed := testPDNSTSIGKeyConfigWriteOnlyNamed("tf-wo-renamed", testTSIGSecretRotated, 1)
	rotated := testPDNSTSIGKeyConfigWriteOnlyNamed("tf-wo-renamed", testTSIGSecret, 2)
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: initial,
				Check:  testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
			},
			{
				// A rename replaces the key and sends the currently configured
				// secret even though its version has not changed.
				Config: renamed,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", "tf-wo-renamed."),
					resource.TestCheckResourceAttr(resourceName, "name", "tf-wo-renamed"),
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecretRotated),
					testAccCheckPDNSTSIGKeyNoPersistedSecret(resourceName),
					resource.TestCheckNoResourceAttr(resourceName, "key_wo"),
				),
			},
			{Config: renamed, PlanOnly: true},
			{
				Config: rotated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
					testAccCheckPDNSTSIGKeyNoPersistedSecret(resourceName),
				),
			},
			{Config: rotated, PlanOnly: true},
		},
	})
}

func TestAccPDNSTSIGKeyTrailingDot(t *testing.T) {
	const resourceName = "powerdns_tsigkey.test-dotted"
	config := func(name string) string {
		return fmt.Sprintf(`
resource "powerdns_tsigkey" "test-dotted" {
  name      = %q
  algorithm = "hmac-sha256"
  key       = %q
}`, name, testTSIGSecret)
	}
	initial := config("tf-dotted.")
	renamed := config("tf-dotted-renamed.")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: initial,
				Check: resource.ComposeTestCheckFunc(
					// State keeps the spelling from the configuration.
					resource.TestCheckResourceAttr(resourceName, "name", "tf-dotted."),
					resource.TestCheckResourceAttr(resourceName, "id", "tf-dotted."),
				),
			},
			{Config: initial, PlanOnly: true},
			{
				// Dropping the trailing dot is the same key: it is updated in
				// place and never replaced.
				Config: config("tf-dotted"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-dotted"),
					resource.TestCheckResourceAttr(resourceName, "id", "tf-dotted."),
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
				),
			},
			{Config: config("tf-dotted"), PlanOnly: true},
			{ResourceName: resourceName, ImportState: true, ImportStateVerify: true},
			{
				Config: renamed,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-dotted-renamed."),
					resource.TestCheckResourceAttr(resourceName, "id", "tf-dotted-renamed."),
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
				),
			},
			{Config: renamed, PlanOnly: true},
		},
	})
}

func TestAccPDNSTSIGKeyExplicitSecret(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-explicit"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigExplicit,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "tf-explicit"),
					resource.TestCheckResourceAttr(resourceName, "key", testTSIGSecret),
				),
			},
			{
				// Rotating the secret is an in-place update.
				Config: testPDNSTSIGKeyConfigExplicitRotated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "key", testTSIGSecretRotated),
				),
			},
		},
	})
}

func TestAccPDNSTSIGKeyAlgorithmForcesNew(t *testing.T) {
	resourceName := "powerdns_tsigkey.test-algorithm"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSTSIGKeyConfigAlgorithm,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "hmac-sha256"),
				),
			},
			{
				// PowerDNS would otherwise keep the old algorithm alongside the
				// new one under the same name, so this must replace the key.
				Config: testPDNSTSIGKeyConfigAlgorithmChanged,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeyExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "algorithm", "hmac-sha512"),
				),
			},
		},
	})
}

func TestAccPDNSTSIGKeyInvalidAlgorithm(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSTSIGKeyConfigInvalidAlgorithm,
				ExpectError: regexp.MustCompile(`algorithm\s+value\s+must\s+be\s+one\s+of`),
			},
		},
	})
}

func testAccCheckPDNSTSIGKeyDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "powerdns_tsigkey" {
			continue
		}

		client := testAccProvider.Meta().(*ProviderClients)
		_, err := client.PDNS.GetTSIGKey(context.Background(), rs.Primary.ID, rs.Primary.Attributes["algorithm"])
		if err == nil {
			return fmt.Errorf("TSIG key %s still exists", rs.Primary.ID)
		}
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("error checking TSIG key %s is gone: %w", rs.Primary.ID, err)
		}
	}

	return nil
}

func testAccCheckPDNSTSIGKeyExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no TSIG key ID is set")
		}

		client := testAccProvider.Meta().(*ProviderClients)
		if _, err := client.PDNS.GetTSIGKey(context.Background(), rs.Primary.ID, rs.Primary.Attributes["algorithm"]); err != nil {
			return fmt.Errorf("error fetching TSIG key %s: %w", rs.Primary.ID, err)
		}

		return nil
	}
}

const testTSIGSecret = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3OD0="
const testTSIGSecretRotated = "MTIzNDU2Nzg5MGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHk="

const testPDNSTSIGKeyConfigGenerated = `
resource "powerdns_tsigkey" "test-generated" {
	name      = "tf-generated"
	algorithm = "hmac-sha256"
}`

const testPDNSTSIGKeyConfigExplicit = `
resource "powerdns_tsigkey" "test-explicit" {
	name      = "tf-explicit"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecret + `"
}`

const testPDNSTSIGKeyConfigExplicitRotated = `
resource "powerdns_tsigkey" "test-explicit" {
	name      = "tf-explicit"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecretRotated + `"
}`

const testPDNSTSIGKeyConfigAlgorithm = `
resource "powerdns_tsigkey" "test-algorithm" {
	name      = "tf-algorithm"
	algorithm = "hmac-sha256"
	key       = "` + testTSIGSecret + `"
}`

const testPDNSTSIGKeyConfigAlgorithmChanged = `
resource "powerdns_tsigkey" "test-algorithm" {
	name      = "tf-algorithm"
	algorithm = "hmac-sha512"
	key       = "` + testTSIGSecret + `"
}`

const testPDNSTSIGKeyConfigInvalidAlgorithm = `
resource "powerdns_tsigkey" "test-invalid" {
	name      = "tf-invalid"
	algorithm = "hmac-nonsense"
}`

func testAccCheckPDNSTSIGKeyNoPersistedSecret(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		// The SDK serializes a cleared optional/computed string as empty.
		// Both empty and absent values mean that no secret is persisted.
		if rs.Primary.Attributes["key"] != "" {
			return fmt.Errorf("%s: secret material remains in key state during write-only mode", n)
		}
		return nil
	}
}

// testAccCheckPDNSTSIGKeySecret asks the server, since state must not hold a
// write-only secret.
func testAccCheckPDNSTSIGKeySecret(n string, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		client := testAccProvider.Meta().(*ProviderClients)
		got, err := client.PDNS.GetTSIGKey(context.Background(), rs.Primary.ID, rs.Primary.Attributes["algorithm"])
		if err != nil {
			return fmt.Errorf("error fetching TSIG key %s: %w", rs.Primary.ID, err)
		}
		if got.Key != want {
			return fmt.Errorf("server holds a different secret than the configured write-only value")
		}
		return nil
	}
}

func testPDNSTSIGKeyConfigWriteOnly(secret string, version int) string {
	return testPDNSTSIGKeyConfigWriteOnlyNamed("tf-write-only", secret, version)
}

func testPDNSTSIGKeyConfigWriteOnlyNamed(name, secret string, version int) string {
	return fmt.Sprintf(`
resource "powerdns_tsigkey" "test-wo" {
	name           = %q
	algorithm      = "hmac-sha256"
	key_wo         = %q
	key_wo_version = %d
}`, name, secret, version)
}

func TestTSIGKeyCreateKeepsStateWhenReadFails(t *testing.T) {
	client := newTestClient(writesSucceedReadsFail(http.StatusCreated, `{"id":"kept.","name":"kept","algorithm":"hmac-sha256"}`, 0, ""))
	assertCreateKeepsState(t, newTSIGKeyResource(), &ProviderClients{PDNS: client},
		map[string]tftypes.Value{"name": tfString("kept"), "algorithm": tfString("hmac-sha256")},
		"kept.")
}
