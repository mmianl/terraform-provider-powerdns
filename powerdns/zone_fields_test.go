package powerdns

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
			config := map[string]tftypes.Value{"name": tfString("fields.example."), "kind": tfString("Native")}
			if configured != nil {
				config["api_rectify"] = tfBool(configured.(bool))
			}
			h := newResourceHarness(t, newZoneResource(), &ProviderClients{PDNS: client})
			_, diags := h.create(config)
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
	zone := map[string]interface{}{
		"id": "fields.example.", "name": "fields.example.", "kind": "Native", "account": "admin",
		"dnssec": true, "api_rectify": true, "soa_edit": "INCEPTION-INCREMENT",
		"soa_edit_api": "DEFAULT", "serial": 42, "edited_serial": 43, "notified_serial": 40, "last_check": 7,
		"master_tsig_key_ids": []string{"transfer."}, "slave_tsig_key_ids": []string{"transfer."},
	}
	var payload map[string]interface{}
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			payload = nil
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
	h := newResourceHarness(t, newZoneResource(), &ProviderClients{PDNS: client})

	state, diags := h.importState("fields.example.")
	if !assert.False(t, diags.HasError(), "%v", diags) {
		t.FailNow()
	}

	config := map[string]tftypes.Value{
		"name": tfString("fields.example."), "kind": tfString("Native"), "account": tfString("changed"),
	}
	state, diags = h.update(state, config)
	assert.False(t, diags.HasError(), "%v", diags)
	for _, name := range []string{"dnssec", "api_rectify", "soa_edit", "master_tsig_key_ids", "slave_tsig_key_ids"} {
		assert.NotContains(t, payload, name, "an unrelated update must omit %s", name)
	}
	assert.Equal(t, true, zone["dnssec"])
	assert.Equal(t, true, zone["api_rectify"])

	config["dnssec"], config["api_rectify"], config["soa_edit"] = tfBool(false), tfBool(false), tfString("INCEPTION")
	state, diags = h.update(state, config)
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, false, payload["dnssec"])
	assert.Equal(t, false, payload["api_rectify"])
	assert.Equal(t, "INCEPTION", payload["soa_edit"])
	assert.NotContains(t, payload, "master_tsig_key_ids")
	assert.NotContains(t, payload, "slave_tsig_key_ids")

	for name, want := range map[string]int64{"serial": 42, "edited_serial": 43, "notified_serial": 40, "last_check": 7} {
		assert.Equal(t, want, tfAttrInt(t, state, name), name)
	}
	soaEdit := tfAttrString(t, state, "soa_edit")
	assert.Equal(t, "INCEPTION", soaEdit)
}

func TestZoneDataSourceFields(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"name":"fields.example.","kind":"Master","catalog":"catalog.example.","soa_edit":"INCEPTION-INCREMENT","dnssec":true,"api_rectify":true,"serial":42,"edited_serial":43,"notified_serial":40,"last_check":7,"rrsets":[]}`), nil
	})
	state, diags := readDataSource(t, newZoneDataSource(), &ProviderClients{PDNS: client},
		map[string]tftypes.Value{"name": tfString("fields.example.")})
	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, "catalog.example.", tfAttrString(t, state, "catalog"))
	assert.Equal(t, "INCEPTION-INCREMENT", tfAttrString(t, state, "soa_edit"))
	dnssec, _ := tfAttrBool(t, state, "dnssec")
	rectify, _ := tfAttrBool(t, state, "api_rectify")
	assert.True(t, dnssec)
	assert.True(t, rectify)
	for name, want := range map[string]int64{"serial": 42, "edited_serial": 43, "notified_serial": 40, "last_check": 7} {
		assert.Equal(t, want, tfAttrInt(t, state, name), name)
	}
}
