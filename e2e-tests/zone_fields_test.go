package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

type zoneFields struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Account        string `json:"account"`
	Catalog        string `json:"catalog"`
	DNSSEC         *bool  `json:"dnssec"`
	APIRectify     *bool  `json:"api_rectify"`
	SOAEdit        string `json:"soa_edit"`
	Serial         *int64 `json:"serial"`
	EditedSerial   *int64 `json:"edited_serial"`
	NotifiedSerial *int64 `json:"notified_serial"`
	LastCheck      *int64 `json:"last_check"`
}

func assertZoneFields(t *testing.T, value, live zoneFields, dnssec, rectify bool, soaEdit string) {
	t.Helper()
	if value.ID != live.ID || value.Name != live.Name || value.Account != live.Account ||
		value.DNSSEC == nil || *value.DNSSEC != dnssec || value.APIRectify == nil || *value.APIRectify != rectify || value.SOAEdit != soaEdit {
		t.Fatalf("unexpected zone fields: %+v; live=%+v", value, live)
	}
	// Pointers distinguish an exported zero from a missing attribute. Compare
	// each serial with the API rather than assuming SOA edit leaves it unchanged.
	for name, pair := range map[string][2]*int64{
		"serial":          {value.Serial, live.Serial},
		"edited_serial":   {value.EditedSerial, live.EditedSerial},
		"notified_serial": {value.NotifiedSerial, live.NotifiedSerial},
		"last_check":      {value.LastCheck, live.LastCheck},
	} {
		if pair[0] == nil || pair[1] == nil {
			t.Fatalf("%s is missing from Terraform or the live API", name)
		}
		if *pair[0] != *pair[1] {
			t.Fatalf("%s differs from the live API: got %d, want %d", name, *pair[0], *pair[1])
		}
	}
	if *value.Serial <= 0 || *value.EditedSerial <= 0 || *value.NotifiedSerial != 0 || *value.LastCheck != 0 {
		t.Fatal("expected positive SOA serials and zero notification/freshness fields for a Native zone")
	}
}

func TestPowerDNSZoneFields(t *testing.T) {
	const address = "powerdns_zone.fields"
	const zoneName = "zone-fields.example.com."
	const zonePath = "/api/v1/servers/localhost/zones/" + zoneName
	// Use the same shared configuration and scoped plans as other lifecycle
	// tests, avoiding unrelated Recursor hostname/IP drift.
	targets := []string{
		"-target=" + address, "-target=powerdns_zone.fields_disabled",
		"-target=data.powerdns_zone.fields", "-target=data.powerdns_zone.fields_disabled",
	}
	var flags []string
	args := func(command string, options ...string) []string {
		result := append([]string{command, "-input=false", "-no-color"}, options...)
		result = append(result, targets...)
		return append(result, flags...)
	}
	stablePlan := func() {
		t.Helper()
		runTerraform(t, args("plan", "-detailed-exitcode")...)
	}
	planPath := filepath.Join(t.TempDir(), "zone-fields.tfplan")
	applyUpdate := func() {
		t.Helper()
		runTerraform(t, args("plan", "-out="+planPath)...)
		var plan struct {
			Resources []struct {
				Address string `json:"address"`
				Mode    string `json:"mode"`
				Change  struct {
					Actions []string `json:"actions"`
				} `json:"change"`
			} `json:"resource_changes"`
		}
		out, err := exec.Command("terraform", "show", "-json", planPath).CombinedOutput()
		if err != nil {
			t.Fatalf("reading zone fields plan: %v", err)
		}
		if err := json.Unmarshal(out, &plan); err != nil {
			t.Fatal(err)
		}
		changes := map[string][]string{}
		for _, resource := range plan.Resources {
			if resource.Mode == "managed" && !slices.Equal(resource.Change.Actions, []string{"no-op"}) {
				changes[resource.Address] = resource.Change.Actions
			}
		}
		if !reflect.DeepEqual(changes, map[string][]string{address: {"update"}}) {
			t.Fatalf("expected only an in-place zone update, got %v", changes)
		}
		runTerraform(t, "apply", "-input=false", "-no-color", planPath)
	}
	stateValues := func() map[string]zoneFields {
		t.Helper()
		var state struct {
			Values struct {
				RootModule struct {
					Resources []struct {
						Address string     `json:"address"`
						Values  zoneFields `json:"values"`
					} `json:"resources"`
				} `json:"root_module"`
			} `json:"values"`
		}
		tsigTerraformJSON(t, &state, "show", "-json")
		result := map[string]zoneFields{}
		for _, resource := range state.Values.RootModule.Resources {
			result[resource.Address] = resource.Values
		}
		return result
	}
	check := func(dnssec, rectify bool, soaEdit, account string) zoneFields {
		t.Helper()
		var output struct {
			Catalog zoneFields `json:"catalog"`
		}
		terraformOutput(t, "powerdns_zone_fields", &output)
		state := stateValues()
		var live zoneFields
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), zonePath), &live)
		if live.Name != zoneName || live.ID != zoneName || live.Account != account {
			t.Fatalf("unexpected live zone identity: %+v", live)
		}
		for _, value := range []zoneFields{live, state[address], state["data.powerdns_zone.fields"]} {
			assertZoneFields(t, value, live, dnssec, rectify, soaEdit)
		}
		var records struct {
			RRSets []authRRSet `json:"rrsets"`
		}
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), zonePath), &records)
		found := false
		for _, rrset := range records.RRSets {
			found = found || (rrset.Name == "www."+zoneName && rrset.Type == "A" && len(rrset.Records) == 1 && rrset.Records[0].Content == "192.0.2.42")
		}
		if !found {
			t.Fatal("changing zone settings removed the existing record")
		}
		var disabled zoneFields
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/zone-fields-disabled.example.com."), &disabled)
		for _, value := range []zoneFields{disabled, state["powerdns_zone.fields_disabled"], state["data.powerdns_zone.fields_disabled"]} {
			assertZoneFields(t, value, disabled, false, false, "NONE")
		}
		var member zoneFields
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/catalog-member.example.com."), &member)
		if output.Catalog.Name != member.Name || output.Catalog.Catalog != "catalog-a.example." || output.Catalog.Catalog != member.Catalog {
			t.Fatal("zone data source did not export the live catalog membership")
		}
		return live
	}
	keys := func() map[int]string {
		t.Helper()
		var live []authCryptokey
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), zonePath+"/cryptokeys"), &live)
		result := map[int]string{}
		for _, key := range live {
			result[key.ID] = key.DNSKey
		}
		if len(result) == 0 {
			t.Fatal("enabling DNSSEC did not generate signing keys")
		}
		return result
	}

	t.Log("Verify enabled and explicitly disabled settings, computed serials, and catalog data")
	// The dependent record bumps the serial after the zone's create read.
	// Refresh computed fields before comparing them with the current API state.
	runTerraform(t, args("apply", "-refresh-only", "-auto-approve")...)
	live := check(true, true, "INCEPTION-INCREMENT", "admin")
	initialKeys := keys()
	stablePlan()

	t.Log("Import preserves zone settings, computed fields, and signing keys")
	runTerraform(t, "state", "rm", address)
	runTerraform(t, "import", "-input=false", "-no-color", address, zoneName)
	// Check imported state before any apply could restore configured values.
	assertZoneFields(t, stateValues()[address], live, true, true, "INCEPTION-INCREMENT")
	runTerraform(t, args("apply", "-auto-approve")...)
	check(true, true, "INCEPTION-INCREMENT", "admin")
	if !reflect.DeepEqual(keys(), initialKeys) {
		t.Fatal("import changed existing signing keys")
	}
	stablePlan()

	t.Log("An unrelated account update preserves omitted optional settings and signing keys")
	flags = []string{"-var=zone_fields_manage_settings=false", "-var=zone_fields_account=updated-account"}
	applyUpdate()
	check(true, true, "INCEPTION-INCREMENT", "updated-account")
	if !reflect.DeepEqual(keys(), initialKeys) {
		t.Fatal("an unrelated zone update changed signing keys")
	}
	stablePlan()

	t.Log("Disable DNSSEC and automatic rectification and update the SOA edit rule in place")
	flags = []string{"-var=zone_fields_account=updated-account", "-var=zone_fields_dnssec=false", "-var=zone_fields_api_rectify=false", "-var=zone_fields_soa_edit=NONE"}
	applyUpdate()
	check(false, false, "NONE", "updated-account")
	stablePlan()

	t.Log("Re-enable DNSSEC independently of automatic rectification")
	flags = []string{"-var=zone_fields_account=updated-account", "-var=zone_fields_api_rectify=false", "-var=zone_fields_soa_edit=NONE"}
	applyUpdate()
	check(true, false, "NONE", "updated-account")
	keys()
	stablePlan()
}
