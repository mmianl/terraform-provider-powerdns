package powerdns

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestZoneFieldsCreateDefaults(t *testing.T) {
	for name, configured := range map[string]interface{}{"omitted": nil, "false": false, "true": true} {
		t.Run(name, func(t *testing.T) {
			var payload map[string]interface{}
			client := newTestClient(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPost {
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					return jsonResponse(http.StatusCreated, `{"id":"fields.example."}`), nil
				}
				return jsonResponse(http.StatusOK, `{"name":"fields.example.","kind":"Native","api_rectify":true}`), nil
			})
			config := map[string]interface{}{"name": "fields.example.", "kind": "Native"}
			if configured != nil {
				config["api_rectify"] = configured
			}
			res := resourcePDNSZone()
			diff, err := res.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(config), nil)
			if !assert.NoError(t, err) || !assert.NotNil(t, diff) {
				t.FailNow()
			}
			value := cty.NullVal(cty.Bool)
			if configured != nil {
				value = cty.BoolVal(configured.(bool))
			}
			diff.RawConfig = cty.ObjectVal(map[string]cty.Value{"api_rectify": value})
			_, diags := res.Apply(context.Background(), nil, diff, &ProviderClients{PDNS: client})
			assert.False(t, diags.HasError(), "%v", diags)
			if configured == nil {
				assert.NotContains(t, payload, "api_rectify", "omission must preserve the server default")
			} else {
				assert.Equal(t, configured, payload["api_rectify"])
			}
		})
	}
}

func TestZoneFieldsUpdatePreservesUnmanagedSettings(t *testing.T) {
	res := resourcePDNSZone()
	zone := map[string]interface{}{
		"id": "fields.example.", "name": "fields.example.", "kind": "Native", "account": "admin",
		"dnssec": true, "api_rectify": true, "soa_edit": "INCEPTION-INCREMENT",
		"soa_edit_api": "DEFAULT", "serial": 42, "edited_serial": 43, "notified_serial": 40, "last_check": 7,
		"master_tsig_key_ids": []string{"transfer."}, "slave_tsig_key_ids": []string{"transfer."},
	}
	var payload map[string]interface{}
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			for name, value := range payload {
				zone[name] = value
			}
			return jsonResponse(http.StatusNoContent, ""), nil
		}
		assert.Equal(t, http.MethodGet, r.Method)
		body, err := json.Marshal(zone)
		assert.NoError(t, err)
		return jsonResponse(http.StatusOK, string(body)), nil
	})
	meta := &ProviderClients{PDNS: client}
	state, diags := res.RefreshWithoutUpgrade(context.Background(), &terraform.InstanceState{ID: "fields.example."}, meta)
	if !assert.False(t, diags.HasError(), "%v", diags) {
		t.FailNow()
	}
	config := map[string]interface{}{"name": "fields.example.", "kind": "Native", "account": "changed"}
	apply := func() {
		t.Helper()
		diff, err := res.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
		if !assert.NoError(t, err) || !assert.NotNil(t, diff) {
			t.FailNow()
		}
		for _, change := range diff.Attributes {
			assert.False(t, change.RequiresNew, "zone settings must update in place")
		}
		state, diags = res.Apply(context.Background(), state, diff, meta)
		assert.False(t, diags.HasError(), "%v", diags)
	}
	apply()
	for _, name := range []string{"dnssec", "api_rectify", "soa_edit", "master_tsig_key_ids", "slave_tsig_key_ids"} {
		assert.NotContains(t, payload, name, "an unrelated update must omit %s", name)
	}
	assert.Equal(t, true, zone["dnssec"])
	assert.Equal(t, true, zone["api_rectify"])

	config["dnssec"], config["api_rectify"], config["soa_edit"] = false, false, "INCEPTION"
	apply()
	assert.Equal(t, false, payload["dnssec"])
	assert.Equal(t, false, payload["api_rectify"])
	assert.Equal(t, "INCEPTION", payload["soa_edit"])
	assert.NotContains(t, payload, "master_tsig_key_ids")
	assert.NotContains(t, payload, "slave_tsig_key_ids")
	diff, err := res.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
	assert.NoError(t, err)
	if diff != nil {
		assert.True(t, diff.Empty(), "updated settings must converge")
	}
	for name, want := range map[string]string{"serial": "42", "edited_serial": "43", "notified_serial": "40", "last_check": "7"} {
		assert.Equal(t, want, state.Attributes[name])
	}
}

func TestZoneDataSourceFields(t *testing.T) {
	res := dataSourcePDNSZone()
	data := schema.TestResourceDataRaw(t, res.Schema, map[string]interface{}{"name": "fields.example."})
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"name":"fields.example.","kind":"Master","catalog":"catalog.example.","soa_edit":"INCEPTION-INCREMENT","dnssec":true,"api_rectify":true,"serial":42,"edited_serial":43,"notified_serial":40,"last_check":7,"rrsets":[]}`), nil
	})
	diags := dataSourcePDNSZoneRead(context.Background(), data, &ProviderClients{PDNS: client})
	assert.False(t, diags.HasError(), "%v", diags)
	for name, want := range map[string]interface{}{
		"catalog": "catalog.example.", "soa_edit": "INCEPTION-INCREMENT", "dnssec": true,
		"api_rectify": true, "serial": 42, "edited_serial": 43, "notified_serial": 40, "last_check": 7,
	} {
		assert.Equal(t, want, data.Get(name), name)
	}
}
