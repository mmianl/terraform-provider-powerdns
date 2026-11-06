package powerdns

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeTSIGKeyID(t *testing.T) {
	for input, want := range map[string]string{"": "", "transfer": "transfer.", "transfer.": "transfer."} {
		assert.Equal(t, want, NormalizeTSIGKeyID(input))
	}
}

func TestZoneTSIGReferenceSetNormalization(t *testing.T) {
	res := resourcePDNSZone()
	for _, name := range []string{"master_tsig_key_ids", "slave_tsig_key_ids"} {
		t.Run(name, func(t *testing.T) {
			field := res.Schema[name]
			assert.Equal(t, field.Set("transfer"), field.Set("transfer."))
			data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{
				"name": "refs.example.com.", "kind": "Master", name: []interface{}{"transfer", "transfer."},
			})
			assert.Equal(t, []interface{}{"transfer."}, data.Get(name).(*schema.Set).List())
		})
	}
}

func TestZoneTSIGReferencesLifecycle(t *testing.T) {
	res := resourcePDNSZone()
	var zone ZoneInfo
	var payloads []map[string]json.RawMessage
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.Method {
		case http.MethodPost:
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&zone))
			zone.ID = zone.Name
			zone.SoaEditAPI = "DEFAULT"
		case http.MethodPut:
			var payload map[string]json.RawMessage
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			payloads = append(payloads, payload)
			for name, target := range map[string]*[]string{
				"master_tsig_key_ids": &zone.MasterTsigKeyIDs,
				"slave_tsig_key_ids":  &zone.SlaveTsigKeyIDs,
			} {
				if raw, ok := payload[name]; ok {
					assert.NoError(t, json.Unmarshal(raw, target))
				}
			}
			assert.NoError(t, json.Unmarshal(payload["account"], &zone.Account))
			return jsonResponse(http.StatusNoContent, ""), nil
		default:
			assert.Equal(t, http.MethodGet, r.Method)
		}
		body, err := json.Marshal(zone)
		assert.NoError(t, err)
		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusCreated
		}
		return jsonResponse(status, string(body)), nil
	})
	config := func(refs []string, include bool, account string) map[string]interface{} {
		values := map[string]interface{}{"name": "refs.example.com.", "kind": "Master", "account": account}
		if include {
			items := make([]interface{}, len(refs))
			for i, ref := range refs {
				items[i] = ref
			}
			values["master_tsig_key_ids"] = items
			values["slave_tsig_key_ids"] = items
		}
		return values
	}
	var state *terraform.InstanceState
	apply := func(values map[string]interface{}) {
		t.Helper()
		diff, err := res.Diff(context.Background(), state, terraform.NewResourceConfigRaw(values), nil)
		if !assert.NoError(t, err) || !assert.NotNil(t, diff) {
			t.FailNow()
		}
		assert.False(t, diff.Empty())
		// Use the real SDK lifecycle so the tests cover normalization, payloads,
		// and refresh together, including Optional+Computed set behavior.
		updated, diagnostics := res.Apply(context.Background(), state, diff, &ProviderClients{PDNS: client})
		assert.False(t, diagnostics.HasError(), "%v", diagnostics)
		state = updated
	}
	assertEmptyPlan := func(values map[string]interface{}) {
		t.Helper()
		diff, err := res.Diff(context.Background(), state, terraform.NewResourceConfigRaw(values), nil)
		assert.NoError(t, err)
		if diff != nil {
			assert.True(t, diff.Empty(), "unexpected diff: %#v", diff.Attributes)
		}
	}

	apply(config([]string{"transfer"}, true, "admin"))
	assert.Equal(t, []string{"transfer."}, zone.MasterTsigKeyIDs)
	assert.Equal(t, []string{"transfer."}, zone.SlaveTsigKeyIDs)
	assertEmptyPlan(config([]string{"transfer."}, true, "admin"))
	assertEmptyPlan(config([]string{"transfer", "transfer."}, true, "admin"))

	apply(config([]string{"replacement"}, true, "admin"))
	assert.Equal(t, []string{"replacement."}, zone.MasterTsigKeyIDs)
	assert.Equal(t, []string{"replacement."}, zone.SlaveTsigKeyIDs)
	assertEmptyPlan(config([]string{"replacement"}, true, "admin"))

	apply(config([]string{}, true, "admin"))
	last := payloads[len(payloads)-1]
	assert.JSONEq(t, "[]", string(last["master_tsig_key_ids"]))
	assert.JSONEq(t, "[]", string(last["slave_tsig_key_ids"]))
	assert.Empty(t, zone.MasterTsigKeyIDs)
	assert.Empty(t, zone.SlaveTsigKeyIDs)
	assertEmptyPlan(config([]string{}, true, "admin"))

	apply(config([]string{"external"}, true, "admin"))
	apply(config(nil, false, "updated-account"))
	last = payloads[len(payloads)-1]
	assert.NotContains(t, last, "master_tsig_key_ids")
	assert.NotContains(t, last, "slave_tsig_key_ids")
	assert.Equal(t, []string{"external."}, zone.MasterTsigKeyIDs)
	assert.Equal(t, []string{"external."}, zone.SlaveTsigKeyIDs)
	assertEmptyPlan(config(nil, false, "updated-account"))

	imported, diags := res.RefreshWithoutUpgrade(context.Background(), &terraform.InstanceState{ID: zone.ID}, &ProviderClients{PDNS: client})
	assert.False(t, diags.HasError(), "%v", diags)
	state = imported
	assertEmptyPlan(config([]string{"external"}, true, "updated-account"))
}

func TestZoneDataSourceTSIGReferences(t *testing.T) {
	res := dataSourcePDNSZone()
	data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "refs.example.com."})
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		return jsonResponse(http.StatusOK, `{"name":"refs.example.com.","kind":"Master","master_tsig_key_ids":["transfer."],"slave_tsig_key_ids":["transfer."],"rrsets":[]}`), nil
	})
	diags := dataSourcePDNSZoneRead(context.Background(), data, &ProviderClients{PDNS: client})
	assert.False(t, diags.HasError(), "%v", diags)
	for _, name := range []string{"master_tsig_key_ids", "slave_tsig_key_ids"} {
		assert.Equal(t, []interface{}{"transfer."}, data.Get(name).(*schema.Set).List())
	}
}

func TestZoneTSIGReferencesSecondaryLimit(t *testing.T) {
	diags := resourcePDNSZone().Validate(terraform.NewResourceConfigRaw(map[string]interface{}{
		"name": "refs.example.com.", "kind": "Master", "slave_tsig_key_ids": []interface{}{"one", "two"},
	}))
	assert.True(t, diags.HasError())
	found := false
	for _, diagnostic := range diags {
		found = found || strings.Contains(diagnostic.Summary+diagnostic.Detail, "slave_tsig_key_ids")
	}
	assert.True(t, found, "validation must identify the unsupported secondary key count")
}
