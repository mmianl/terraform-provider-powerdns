package powerdns

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeTSIGKeyID(t *testing.T) {
	for input, want := range map[string]string{"": "", "transfer": "transfer.", "transfer.": "transfer."} {
		assert.Equal(t, want, NormalizeTSIGKeyID(input))
	}
}

// A bare name and the dotted ID PowerDNS returns are the same reference, so a
// refresh keeps the spelling written in the configuration, including a set that
// names the same key twice.
func TestZoneTSIGReferenceSetKeepsConfiguredSpelling(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics
	held := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("transfer"), types.StringValue("transfer.")})

	got := tsigReferenceSet(ctx, held, []string{"transfer."}, &diags)
	assert.False(t, diags.HasError())
	assert.True(t, got.Equal(held), "equivalent spelling must be kept")

	got = tsigReferenceSet(ctx, held, []string{"replacement."}, &diags)
	assert.Equal(t, []string{"replacement."}, tsigReferences(ctx, got, &diags))

	got = tsigReferenceSet(ctx, types.SetNull(types.StringType), []string{"transfer."}, &diags)
	assert.Equal(t, []string{"transfer."}, tsigReferences(ctx, got, &diags))
}

func TestZoneTSIGReferencesLifecycle(t *testing.T) {
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
	h := newResourceHarness(t, newZoneResource(), &ProviderClients{PDNS: client})

	config := func(refs []string, include bool, account string) map[string]tftypes.Value {
		values := map[string]tftypes.Value{
			"name": tfString("refs.example.com."), "kind": tfString("Master"), "account": tfString(account),
		}
		if include {
			values["master_tsig_key_ids"] = tfStringSet(refs...)
			values["slave_tsig_key_ids"] = tfStringSet(refs...)
		}
		return values
	}

	state, diags := h.create(config([]string{"transfer"}, true, "admin"))
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"transfer."}, zone.MasterTsigKeyIDs)
	assert.Equal(t, []string{"transfer."}, zone.SlaveTsigKeyIDs)

	state, diags = h.update(state, config([]string{"replacement"}, true, "admin"))
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"replacement."}, zone.MasterTsigKeyIDs)
	assert.Equal(t, []string{"replacement."}, zone.SlaveTsigKeyIDs)

	state, diags = h.update(state, config([]string{}, true, "admin"))
	assert.False(t, diags.HasError(), "%v", diags)
	last := payloads[len(payloads)-1]
	assert.JSONEq(t, "[]", string(last["master_tsig_key_ids"]))
	assert.JSONEq(t, "[]", string(last["slave_tsig_key_ids"]))
	assert.Empty(t, zone.MasterTsigKeyIDs)
	assert.Empty(t, zone.SlaveTsigKeyIDs)

	state, diags = h.update(state, config([]string{"external"}, true, "admin"))
	assert.False(t, diags.HasError(), "%v", diags)
	state, diags = h.update(state, config(nil, false, "updated-account"))
	assert.False(t, diags.HasError(), "%v", diags)
	last = payloads[len(payloads)-1]
	assert.NotContains(t, last, "master_tsig_key_ids")
	assert.NotContains(t, last, "slave_tsig_key_ids")
	assert.Equal(t, []string{"external."}, zone.MasterTsigKeyIDs)
	assert.Equal(t, []string{"external."}, zone.SlaveTsigKeyIDs)

	imported, diags := h.importState(zone.ID)
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"external."}, tfAttrStrings(t, imported, "master_tsig_key_ids"))
	assert.Equal(t, []string{"external."}, tfAttrStrings(t, imported, "slave_tsig_key_ids"))
	_ = state
}

func TestZoneDataSourceTSIGReferences(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		return jsonResponse(http.StatusOK, `{"name":"refs.example.com.","kind":"Master","master_tsig_key_ids":["transfer."],"slave_tsig_key_ids":["transfer."],"rrsets":[]}`), nil
	})
	state, diags := readDataSource(t, newZoneDataSource(), &ProviderClients{PDNS: client},
		map[string]tftypes.Value{"name": tfString("refs.example.com.")})
	assert.False(t, diags.HasError(), "%v", diags)
	for _, name := range []string{"master_tsig_key_ids", "slave_tsig_key_ids"} {
		assert.Equal(t, []string{"transfer."}, tfAttrStrings(t, state, name))
	}
}

func TestZoneTSIGReferencesSecondaryLimit(t *testing.T) {
	h := newResourceHarness(t, newZoneResource(), &ProviderClients{})
	attribute, ok := h.schema.Attributes["slave_tsig_key_ids"].(schema.SetAttribute)
	if !assert.True(t, ok) {
		return
	}
	validate := func(values ...string) bool {
		elements := make([]attr.Value, len(values))
		for i, v := range values {
			elements[i] = types.StringValue(v)
		}
		resp := &validator.SetResponse{}
		for _, v := range attribute.Validators {
			v.ValidateSet(context.Background(), validator.SetRequest{
				Path:        path.Root("slave_tsig_key_ids"),
				ConfigValue: types.SetValueMust(types.StringType, elements),
			}, resp)
		}
		return resp.Diagnostics.HasError()
	}
	assert.False(t, validate("one"))
	assert.True(t, validate("one", "two"), "validation must reject more than one secondary key")
}
