package powerdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestTSIGKeyUpdateSendsSecretOnlyForRotation(t *testing.T) {
	tests := []struct {
		name           string
		oldVersion     int
		newVersion     int
		rename         bool
		storedRotation bool
		wantKey        bool
	}{
		{name: "write-only rename", oldVersion: 1, newVersion: 1, rename: true},
		{name: "write-only rename and rotation", oldVersion: 1, newVersion: 2, rename: true, wantKey: true},
		{name: "write-only rotation", oldVersion: 1, newVersion: 2, wantKey: true},
		{name: "stored secret rename", rename: true},
		{name: "stored secret rotation", storedRotation: true, wantKey: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &terraform.InstanceState{
				ID: "tf-old.",
				Attributes: map[string]string{
					"name": "tf-old", "algorithm": "hmac-sha256",
					"key_wo_version": fmt.Sprint(tt.oldVersion),
				},
			}
			if tt.oldVersion == 0 {
				state.Attributes["key"] = testTSIGSecret
			}
			diff := &terraform.InstanceDiff{
				Attributes: map[string]*terraform.ResourceAttrDiff{},
				// Deliberately change the write-only input even when its version
				// is unchanged, to ensure a rename does not send that new secret.
				RawConfig: cty.ObjectVal(map[string]cty.Value{"key_wo": cty.StringVal(testTSIGSecretRotated)}),
			}
			name := "tf-old"
			if tt.rename {
				name = "tf-renamed"
				diff.Attributes["name"] = &terraform.ResourceAttrDiff{Old: "tf-old", New: name}
			}
			if tt.newVersion != tt.oldVersion {
				diff.Attributes["key_wo_version"] = &terraform.ResourceAttrDiff{Old: fmt.Sprint(tt.oldVersion), New: fmt.Sprint(tt.newVersion)}
			}
			if tt.storedRotation {
				diff.Attributes["key"] = &terraform.ResourceAttrDiff{Old: testTSIGSecret, New: testTSIGSecretRotated}
			}
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
					if payload["name"] != name {
						t.Error("update did not send the requested name")
					}
				}
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":%q,"name":%q,"algorithm":"hmac-sha256","key":%q}`, name+".", name, secret)), nil
			})
			updated, diags := resourcePDNSTSIGKey().Apply(context.Background(), state, diff, &ProviderClients{PDNS: client})
			if diags.HasError() {
				t.Fatalf("update failed: %v", diags)
			}
			if puts != 1 || updated.ID != name+"." {
				t.Fatalf("update calls=%d id=%q, want one update and id %q", puts, updated.ID, name+".")
			}
		})
	}
}

func TestTSIGKeyNameTrailingDotDoesNotPlanRename(t *testing.T) {
	for _, name := range []string{"transfer-key", "transfer-key.", "renamed-key"} {
		t.Run(name, func(t *testing.T) {
			res := resourcePDNSTSIGKey()
			state := &terraform.InstanceState{
				ID: "transfer-key.",
				Attributes: map[string]string{
					"name": "transfer-key", "algorithm": "hmac-sha256", "key": testTSIGSecret,
				},
			}
			diff, err := res.Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]interface{}{
				"name": name, "algorithm": "hmac-sha256", "key": testTSIGSecret,
			}), nil)
			if err != nil {
				t.Fatal(err)
			}
			changed := diff != nil && !diff.Empty()
			if changed != (name == "renamed-key") {
				t.Fatalf("name %q produces a rename: %t", name, changed)
			}
		})
	}
}

func TestTSIGKeyNameNormalizedInPlan(t *testing.T) {
	diff, err := resourcePDNSTSIGKey().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(map[string]interface{}{
		"name": "transfer-key.", "algorithm": "hmac-sha256",
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if name := diff.Attributes["name"]; name == nil || name.New != "transfer-key" {
		t.Fatal("the planned name must match the API spelling for dependent resources")
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
			state := &terraform.InstanceState{
				ID: "shared-key.",
				Attributes: map[string]string{
					"name": "shared-key", "algorithm": "hmac-sha256", "key": testTSIGSecret,
				},
			}
			result, diags := resourcePDNSTSIGKey().Apply(context.Background(), state, &terraform.InstanceDiff{Destroy: true}, &ProviderClients{PDNS: client})
			if diags.HasError() {
				t.Fatalf("destroy failed: %v", diags)
			}
			if result != nil && result.ID != "" {
				t.Fatal("destroy retained the managed key in state")
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
			res := resourcePDNSTSIGKey()
			data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
				"name": "tf-transition", "algorithm": "hmac-sha256", "key_wo_version": 1,
			})
			data.SetId("tf-transition.")
			// Model a generated or explicit secret retained in prior state.
			if err := data.Set("key", testTSIGSecret); err != nil {
				t.Fatal(err)
			}
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"id":"tf-transition.","name":"tf-transition","algorithm":"hmac-sha256","key":%q}`, returnedSecret)), nil
			})
			state := data.State()
			for i := 0; i < 2; i++ {
				refreshed, diags := res.RefreshWithoutUpgrade(context.Background(), state, &ProviderClients{PDNS: client})
				if diags.HasError() {
					t.Fatalf("refresh failed: %v", diags)
				}
				if refreshed.Attributes["key"] != "" || refreshed.Attributes["key_wo"] != "" {
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
				PreCheck:          func() { testAccPreCheck(t) },
				ProviderFactories: testAccProviderFactories,
				CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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

func TestAccPDNSTSIGKeyWriteOnlyRenameDoesNotRotate(t *testing.T) {
	const resourceName = "powerdns_tsigkey.test-wo"
	initial := testPDNSTSIGKeyConfigWriteOnlyNamed("tf-wo-rename", testTSIGSecret, 1)
	renamed := testPDNSTSIGKeyConfigWriteOnlyNamed("tf-wo-renamed", testTSIGSecretRotated, 1)
	rotated := testPDNSTSIGKeyConfigWriteOnlyNamed("tf-wo-renamed", testTSIGSecretRotated, 2)
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: initial,
				Check:  testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
			},
			{
				// The input secret changes together with the name, but its version
				// stays unchanged: the server must retain the original secret.
				Config: renamed,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", "tf-wo-renamed."),
					resource.TestCheckResourceAttr(resourceName, "name", "tf-wo-renamed"),
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecret),
					testAccCheckPDNSTSIGKeyNoPersistedSecret(resourceName),
					resource.TestCheckNoResourceAttr(resourceName, "key_wo"),
				),
			},
			{Config: renamed, PlanOnly: true},
			{
				Config: rotated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckPDNSTSIGKeySecret(resourceName, testTSIGSecretRotated),
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: initial,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-dotted"),
					resource.TestCheckResourceAttr(resourceName, "id", "tf-dotted."),
				),
			},
			{Config: initial, PlanOnly: true},
			{Config: config("tf-dotted"), PlanOnly: true},
			{ResourceName: resourceName, ImportState: true, ImportStateVerify: true},
			{
				Config: renamed,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-dotted-renamed"),
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckPDNSTSIGKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSTSIGKeyConfigInvalidAlgorithm,
				ExpectError: regexp.MustCompile(`expected algorithm to be one of`),
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
