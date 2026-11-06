package main

import (
	"encoding/json"
	"maps"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestPowerDNSZoneActions(t *testing.T) {
	const zoneName = "zone-actions.example.com."
	targets := []string{
		"-target=powerdns_zone_notify.test",
		"-target=powerdns_zone_rectify.test",
		"-target=powerdns_zone_notify.once",
		"-target=powerdns_zone_rectify.once",
	}
	checkState := func(record string) {
		t.Helper()
		var actions map[string]struct {
			ID       string            `json:"id"`
			Zone     string            `json:"zone"`
			Triggers map[string]string `json:"triggers"`
		}
		terraformOutput(t, "powerdns_zone_actions", &actions)
		if len(actions) != 4 {
			t.Fatalf("expected both actions with and without triggers, got %v", actions)
		}
		for _, name := range []string{"notify", "rectify", "notify_once", "rectify_once"} {
			state := actions[name]
			action := strings.TrimSuffix(name, "_once")
			var wantTriggers map[string]string
			if name == action {
				wantTriggers = map[string]string{"records": record}
			}
			if state.ID != zoneName+":"+action || state.Zone != zoneName || !maps.Equal(state.Triggers, wantTriggers) {
				t.Fatalf("unexpected %s state: %+v, want triggers %v", name, state, wantTriggers)
			}
		}
	}
	checkZone := func(record string) {
		t.Helper()
		var zone struct {
			Name   string      `json:"name"`
			Kind   string      `json:"kind"`
			DNSSEC bool        `json:"dnssec"`
			RRSets []authRRSet `json:"rrsets"`
		}
		path := "/api/v1/servers/localhost/zones/" + zoneName + "?rrsets=true"
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), path), &zone)
		if zone.Name != zoneName || zone.Kind != "Master" || !zone.DNSSEC {
			t.Fatalf("expected a DNSSEC-enabled Master zone, got name=%q kind=%q dnssec=%t", zone.Name, zone.Kind, zone.DNSSEC)
		}
		for _, rrset := range zone.RRSets {
			if rrset.Name == "www."+zoneName && rrset.Type == "A" && len(rrset.Records) == 1 && rrset.Records[0].Content == record {
				return
			}
		}
		t.Fatalf("zone %s does not contain the expected www record %s", zoneName, record)
	}
	plan := func(record string) {
		t.Helper()
		args := append([]string{"plan", "-input=false", "-no-color", "-detailed-exitcode", "-var=zone_actions_record_value=" + record}, targets...)
		runTerraform(t, args...)
	}

	t.Log("Verify notify and rectify state, supported DNSSEC setup, and an unchanged plan")
	checkState("192.0.2.1")
	checkZone("192.0.2.1")
	plan("192.0.2.1")

	t.Log("A record change replaces actions with triggers while leaving one-shot actions unchanged")
	planPath := filepath.Join(t.TempDir(), "zone-actions.tfplan")
	args := append([]string{"plan", "-input=false", "-no-color", "-out=" + planPath, "-var=zone_actions_record_value=192.0.2.2"}, targets...)
	runTerraform(t, args...)
	out, err := exec.Command("terraform", "show", "-json", planPath).CombinedOutput()
	if err != nil {
		t.Fatalf("reading zone action plan: %v\n%s", err, out)
	}
	var changes struct {
		Resources []struct {
			Address string `json:"address"`
			Change  struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal(out, &changes); err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, resource := range changes.Resources {
		if !slices.Equal(resource.Change.Actions, []string{"no-op"}) {
			got[resource.Address] = resource.Change.Actions
		}
	}
	want := map[string][]string{
		"powerdns_record.zone_actions_www": {"update"},
		"powerdns_zone_notify.test":        {"delete", "create"},
		"powerdns_zone_rectify.test":       {"delete", "create"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected record-triggered changes: got %v, want %v", got, want)
	}
	runTerraform(t, "apply", "-input=false", "-no-color", planPath)
	checkState("192.0.2.2")
	checkZone("192.0.2.2")
	plan("192.0.2.2")

	t.Log("Deleting action resources preserves the zone, its DNSSEC setup, and its records")
	args = append([]string{"destroy", "-input=false", "-no-color", "-auto-approve"}, targets...)
	runTerraform(t, args...)
	checkZone("192.0.2.2")
	out, err = exec.Command("terraform", "state", "list").CombinedOutput()
	if err != nil {
		t.Fatalf("reading state after deleting actions: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "powerdns_zone_notify.") || strings.Contains(string(out), "powerdns_zone_rectify.") {
		t.Fatalf("action resources remain in state after deletion:\n%s", out)
	}
}
