package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestTSIGWriteOnlyErrorsRedactSecrets(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for name, secret := range map[string]string{
			"plain":               "invalid-base64-test-secret!",
			"quotes and controls": "invalid-\"key\\with\ncontrols\t!",
		} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				mutations := 0
				client := newTestClient(func(r *http.Request) (*http.Response, error) {
					if r.Method == http.MethodGet {
						return jsonResponse(http.StatusOK, `[{"id":"tf-redacted.","algorithm":"hmac-sha256"}]`), nil
					}
					mutations++
					wantMethod := http.MethodPost
					if operation == "update" {
						wantMethod = http.MethodPut
					}
					assert.Equal(t, wantMethod, r.Method)
					var payload TSIGKey
					if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
						t.FailNow()
					}
					if payload.Key != secret {
						t.Fatal("the mutation did not send the configured write-only value")
					}
					// PowerDNS echoes submitted key material in these errors. Include
					// it twice to catch redaction that replaces only one occurrence.
					message := fmt.Sprintf("TSIG content '%s' cannot be base64-decoded: '%s'", secret, secret)
					body, err := json.Marshal(map[string]string{"error": message})
					assert.NoError(t, err)
					return jsonResponse(http.StatusUnprocessableEntity, string(body)), nil
				})
				res := resourcePDNSTSIGKey()
				var state *terraform.InstanceState
				version := 1
				if operation == "update" {
					version = 2
					state = &terraform.InstanceState{ID: "tf-redacted.", Attributes: map[string]string{
						"name": "tf-redacted", "algorithm": "hmac-sha256", "key_wo_version": "1",
					}}
				}
				diff, err := res.Diff(context.Background(), state, terraform.NewResourceConfigRaw(map[string]interface{}{
					"name": "tf-redacted", "algorithm": "hmac-sha256", "key_wo": secret, "key_wo_version": version,
				}), nil)
				if !assert.NoError(t, err) || !assert.NotNil(t, diff) {
					t.FailNow()
				}
				diff.RawConfig = cty.ObjectVal(map[string]cty.Value{"key_wo": cty.StringVal(secret)})
				_, diags := res.Apply(context.Background(), state, diff, &ProviderClients{PDNS: client})
				assert.True(t, diags.HasError())
				assert.Equal(t, 1, mutations)
				for _, diagnostic := range diags {
					message := diagnostic.Summary + diagnostic.Detail
					quoted := strconv.Quote(secret)
					if strings.Contains(message, secret) || strings.Contains(message, quoted[1:len(quoted)-1]) {
						t.Error("the Terraform diagnostic exposes submitted secret material")
					}
					assert.Contains(t, message, "TSIG key tf-redacted")
					assert.Contains(t, message, "cannot be base64-decoded")
					assert.Equal(t, 2, strings.Count(message, "[REDACTED]"))
				}
			})
		}
	}
}

func TestTSIGMutationDecodeErrorsDoNotExposeResponseValues(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, success := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/success=%t", operation, success), func(t *testing.T) {
				const secret = "12345"
				client := newTestClient(func(r *http.Request) (*http.Response, error) {
					if r.Method == http.MethodGet {
						return jsonResponse(http.StatusOK, `[{"id":"tf-redacted.","algorithm":"hmac-sha256"}]`), nil
					}
					if !success {
						return jsonResponse(http.StatusUnprocessableEntity, `{"error":12345}`), nil
					}
					status := http.StatusOK
					if operation == "create" {
						status = http.StatusCreated
					}
					return jsonResponse(status, `{"key":12345}`), nil
				})
				key := TSIGKey{Name: "tf-redacted", Algorithm: "hmac-sha256", Key: secret}
				var err error
				if operation == "create" {
					_, err = client.CreateTSIGKey(context.Background(), key)
				} else {
					_, err = client.UpdateTSIGKey(context.Background(), "tf-redacted.", "hmac-sha256", key)
				}
				if assert.Error(t, err) {
					assert.NotContains(t, err.Error(), secret)
					assert.Contains(t, err.Error(), "failed to decode")
					assert.Contains(t, err.Error(), "HTTP ")
				}
			})
		}
	}
}

func TestTSIGMutationErrorWithoutSubmittedSecret(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnprocessableEntity, `{"error":"Unsupported algorithm"}`), nil
	})
	_, err := client.CreateTSIGKey(context.Background(), TSIGKey{Name: "tf-generated", Algorithm: "unsupported"})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Unsupported algorithm")
		assert.NotContains(t, err.Error(), "[REDACTED]")
	}
}

func TestTSIGMutationNotFoundPreservesSentinel(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `[{"id":"tf-redacted.","algorithm":"hmac-sha256"}]`), nil
		}
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})
	_, err := client.UpdateTSIGKey(context.Background(), "tf-redacted.", "hmac-sha256", TSIGKey{Key: "invalid-base64!"})
	assert.ErrorIs(t, err, ErrNotFound)
}
