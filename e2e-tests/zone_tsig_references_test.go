package main

import (
	"net/http"
	"path/filepath"
	"slices"
	"testing"
)

type zoneTSIGReferences struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Master []string `json:"master_tsig_key_ids"`
	Slave  []string `json:"slave_tsig_key_ids"`
}

func TestPowerDNSZoneTSIGReferences(t *testing.T) {
	const resource = "powerdns_zone.tsig_references"
	const zone = "tsig-references.example.com."
	// As in the other lifecycle tests, scope plans to these fixtures so the
	// unrelated Recursor hostname/IP drift does not affect the assertions.
	targets := []string{"-target=" + resource, "-target=data.powerdns_zone.tsig_references"}
	mode := "bare"
	args := func(command string, options ...string) []string {
		result := append([]string{command, "-input=false", "-no-color"}, options...)
		result = append(result, targets...)
		return append(result, "-var=tsig_zone_reference_mode="+mode)
	}
	stablePlan := func() {
		t.Helper()
		runTerraform(t, args("plan", "-detailed-exitcode")...)
	}
	planFile := filepath.Join(t.TempDir(), "zone-tsig.tfplan")
	applyUpdate := func() {
		t.Helper()
		runTerraform(t, args("plan", "-out="+planFile)...)
		var plan struct {
			Changes []struct {
				Address string `json:"address"`
				Change  struct {
					Actions []string `json:"actions"`
				} `json:"change"`
			} `json:"resource_changes"`
		}
		tsigTerraformJSON(t, &plan, "show", "-json", planFile)
		found := false
		for _, change := range plan.Changes {
			if change.Address == resource {
				found = true
				if !slices.Equal(change.Change.Actions, []string{"update"}) {
					t.Fatalf("zone TSIG change must update in place, got %v", change.Change.Actions)
				}
			}
		}
		if !found {
			t.Fatal("the plan did not contain the zone TSIG update")
		}
		runTerraform(t, "apply", "-input=false", "-no-color", planFile)
	}
	check := func(want []string) {
		t.Helper()
		var output struct {
			Zone zoneTSIGReferences `json:"zone"`
			Data zoneTSIGReferences `json:"data"`
		}
		terraformOutput(t, "powerdns_zone_tsig_references", &output)
		var live zoneTSIGReferences
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/"+zone), &live)
		for _, value := range []zoneTSIGReferences{output.Zone, output.Data, live} {
			if value.ID != zone || value.Name != zone || !slices.Equal(value.Master, want) || !slices.Equal(value.Slave, want) {
				t.Fatalf("unexpected zone TSIG references: %+v, want %v", value, want)
			}
		}
		var metadata []zoneMetadata
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/"+zone+"/metadata"), &metadata)
		byKind := map[string][]string{}
		for _, item := range metadata {
			byKind[item.Kind] = item.Metadata
		}
		for _, kind := range []string{"TSIG-ALLOW-AXFR", "AXFR-MASTER-TSIG"} {
			if !slices.Equal(byKind[kind], want) {
				t.Fatalf("unexpected %s metadata: got %v, want %v", kind, byKind[kind], want)
			}
		}
	}

	t.Log("Bare names and dotted IDs converge to one canonical reference")
	check([]string{"e2e-zone-transfer."})
	stablePlan()

	t.Log("Import preserves zone TSIG references")
	runTerraform(t, "state", "rm", resource)
	runTerraform(t, "import", "-input=false", "-no-color", resource, zone)
	runTerraform(t, args("apply", "-auto-approve")...)
	check([]string{"e2e-zone-transfer."})
	stablePlan()

	t.Log("Using dotted IDs instead of bare names leaves an empty plan")
	mode = "id"
	stablePlan()

	t.Log("Update both reference sets without replacing the zone")
	mode = "next"
	applyUpdate()
	check([]string{"e2e-zone-transfer-next."})
	stablePlan()

	t.Log("Explicit empty sets remove the server metadata and remain stable")
	mode = "clear"
	applyUpdate()
	check([]string{})
	stablePlan()

	for _, id := range []string{"e2e-zone-transfer.", "e2e-zone-transfer-next."} {
		var key tsigKeyValues
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), tsigKeyPath(id, "hmac-sha256")), &key)
		if key.ID != id {
			t.Fatal("clearing zone references changed a TSIG key")
		}
	}
}
