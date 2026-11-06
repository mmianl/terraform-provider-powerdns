package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type tsigKeyValues struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Algorithm    string          `json:"algorithm"`
	Key          string          `json:"key"`
	KeyWO        json.RawMessage `json:"key_wo"`
	KeyWOVersion int             `json:"key_wo_version"`
}

type tsigResource struct {
	Address string        `json:"address"`
	Type    string        `json:"type"`
	Values  tsigKeyValues `json:"values"`
}

type tsigTerraformValues struct {
	RootModule struct {
		Resources []tsigResource `json:"resources"`
	} `json:"root_module"`
}

type tsigKeyExpectation struct {
	Name, Algorithm, Secret string
	WriteOnlyVersion        int
}

func tsigTerraformJSON(t *testing.T, value interface{}, args ...string) {
	t.Helper()
	out, err := exec.Command("terraform", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("terraform %v failed: %v", args, err)
	}
	// State and plan JSON can contain secrets. Do not print the raw output.
	if err := json.Unmarshal(out, value); err != nil {
		t.Fatalf("decoding terraform %v: %v", args, err)
	}
}

func assertTSIGWriteOnlyInputAbsent(t *testing.T, address string, key tsigKeyValues) {
	t.Helper()
	if len(key.KeyWO) != 0 && string(key.KeyWO) != "null" {
		t.Fatalf("%s persisted its write-only input", address)
	}
}

func checkTSIGFixtures(t *testing.T, want map[string]tsigKeyExpectation) map[string]tsigKeyValues {
	t.Helper()
	var output map[string]tsigKeyValues
	terraformOutput(t, "powerdns_tsigkeys", &output)
	var state struct {
		Values tsigTerraformValues `json:"values"`
	}
	tsigTerraformJSON(t, &state, "show", "-json")
	resources := map[string]tsigKeyValues{}
	for _, resource := range state.Values.RootModule.Resources {
		if resource.Type == "powerdns_tsigkey" {
			resources[resource.Address] = resource.Values
			assertTSIGWriteOnlyInputAbsent(t, resource.Address, resource.Values)
		}
	}
	liveKeys := map[string]tsigKeyValues{}
	for _, name := range []string{"generated", "explicit", "write_only"} {
		expect := want[name]
		address := "powerdns_tsigkey." + name
		key, ok := resources[address]
		if !ok {
			t.Fatalf("%s is absent from state", address)
		}
		canonical := strings.TrimSuffix(expect.Name, ".")
		public := output[name]
		if key.ID != canonical+"." || key.Name != canonical || key.Algorithm != expect.Algorithm ||
			public.ID != key.ID || public.Name != key.Name || public.Algorithm != key.Algorithm {
			t.Fatalf("unexpected identity for %s: state id=%q name=%q algorithm=%q; output id=%q name=%q algorithm=%q", address, key.ID, key.Name, key.Algorithm, public.ID, public.Name, public.Algorithm)
		}
		if public.Key != "" {
			t.Fatalf("public output for %s contains secret material", address)
		}
		var live tsigKeyValues
		path := tsigKeyPath(key.ID, expect.Algorithm)
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), path), &live)
		if live.ID != key.ID || live.Name != key.Name || live.Algorithm != key.Algorithm {
			t.Fatalf("API identity differs from state for %s", address)
		}
		decoded, err := base64.StdEncoding.DecodeString(live.Key)
		if err != nil || len(decoded) == 0 || (expect.Secret != "" && live.Key != expect.Secret) {
			t.Fatalf("API returned unexpected key material for %s", address)
		}
		if key.KeyWOVersion != expect.WriteOnlyVersion {
			t.Fatalf("unexpected write-only version for %s: %d", address, key.KeyWOVersion)
		}
		if expect.WriteOnlyVersion != 0 {
			if key.Key != "" {
				t.Fatalf("%s retained secret material in state", address)
			}
		} else if key.Key != live.Key {
			t.Fatalf("stored key differs from the API for %s", address)
		}
		liveKeys[name] = live
	}
	data, hasData := resources["data.powerdns_tsigkey.generated[0]"]
	if want["generated"].WriteOnlyVersion == 0 {
		live := liveKeys["generated"]
		if !hasData || data.ID != live.ID || data.Name != live.Name || data.Algorithm != live.Algorithm || data.Key != live.Key {
			t.Fatal("TSIG data source differs from the API")
		}
	} else if hasData {
		t.Fatal("the secret-reading data source remains after write-only migration")
	}
	// Replacements must not leave an old algorithm behind under the same name.
	var listed []tsigKeyValues
	doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/tsigkeys"), &listed)
	for name, live := range liveKeys {
		count := 0
		for _, key := range listed {
			if key.ID == live.ID {
				count++
				if key.Algorithm != live.Algorithm || key.Key != "" {
					t.Fatalf("unexpected listed attributes for %s", name)
				}
			}
		}
		if count != 1 {
			t.Fatalf("expected one TSIG key for %s, got %d", name, count)
		}
	}
	return liveKeys
}

func tsigKeyPath(id, algorithm string) string {
	return "/api/v1/servers/localhost/tsigkeys/" + url.PathEscape(id) + "?" + url.Values{"algorithm": {algorithm}}.Encode()
}

func assertTSIGKeyDeleted(t *testing.T, id, algorithm string) {
	t.Helper()
	path := tsigKeyPath(id, algorithm)
	resp, err := http.DefaultClient.Do(newRequest(t, http.MethodGet, authBaseURL(), path))
	if err != nil {
		t.Fatalf("checking deleted TSIG key: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted TSIG key %q (%s) returned HTTP %d, want 404", id, algorithm, resp.StatusCode)
	}
}

func TestPowerDNSTSIGKeys(t *testing.T) {
	secret := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	want := map[string]tsigKeyExpectation{
		"generated":  {Name: "e2e-tsig-generated.", Algorithm: "hmac-sha256"},
		"explicit":   {Name: "e2e-tsig-explicit", Algorithm: "hmac-sha256", Secret: secret("e2e-tsig-explicit-initial-secret")},
		"write_only": {Name: "e2e-tsig-write-only", Algorithm: "hmac-sha256", Secret: secret("e2e-tsig-write-only-initial-secret"), WriteOnlyVersion: 1},
	}
	targets := []string{
		"-target=powerdns_tsigkey.generated", "-target=powerdns_tsigkey.explicit", "-target=powerdns_tsigkey.write_only",
		"-target=data.powerdns_tsigkey.generated", "-target=powerdns_zone_metadata.tsig_transfer",
		"-target=powerdns_zone.tsig_write_only",
	}
	var flags []string
	stablePlan := func() {
		t.Helper()
		args := append([]string{"plan", "-input=false", "-no-color", "-detailed-exitcode"}, targets...)
		runTerraform(t, append(args, flags...)...)
	}
	planFile := filepath.Join(t.TempDir(), "tsig.tfplan")
	apply := func(wantChanges map[string][]string) {
		t.Helper()
		args := append([]string{"plan", "-input=false", "-no-color", "-out=" + planFile}, targets...)
		runTerraform(t, append(args, flags...)...)
		var plan struct {
			Planned tsigTerraformValues `json:"planned_values"`
			Changes []struct {
				Address string `json:"address"`
				Mode    string `json:"mode"`
				Type    string `json:"type"`
				Change  struct {
					Actions []string `json:"actions"`
				} `json:"change"`
			} `json:"resource_changes"`
		}
		tsigTerraformJSON(t, &plan, "show", "-json", planFile)
		for _, resource := range plan.Planned.RootModule.Resources {
			if resource.Type == "powerdns_tsigkey" {
				assertTSIGWriteOnlyInputAbsent(t, resource.Address, resource.Values)
			}
		}
		got := map[string][]string{}
		for _, change := range plan.Changes {
			if change.Mode == "managed" && (change.Type == "powerdns_tsigkey" || change.Address == "powerdns_zone.tsig_write_only") && !slices.Equal(change.Change.Actions, []string{"no-op"}) {
				got[change.Address] = change.Change.Actions
			}
		}
		if !reflect.DeepEqual(got, wantChanges) {
			t.Fatalf("unexpected TSIG resource changes: got %v, want %v", got, wantChanges)
		}
		runTerraform(t, "apply", "-input=false", "-no-color", planFile)
	}
	checkWriteOnlyZone := func(keyID string) {
		t.Helper()
		var output, live zoneTSIGReferences
		terraformOutput(t, "powerdns_zone_tsig_write_only", &output)
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/tsig-write-only.example.com."), &live)
		for _, zone := range []zoneTSIGReferences{output, live} {
			if !slices.Equal(zone.Master, []string{keyID}) || !slices.Equal(zone.Slave, []string{keyID}) {
				t.Fatalf("zone references must follow the key ID in the same apply: got master=%v slave=%v, want %q", zone.Master, zone.Slave, keyID)
			}
		}
	}

	t.Log("Verify generated, explicit, and write-only keys, data-source values, and stable trailing-dot names")
	live := checkTSIGFixtures(t, want)
	checkWriteOnlyZone(live["write_only"].ID)
	generated := want["generated"]
	generated.Secret = live["generated"].Key
	want["generated"] = generated
	stablePlan()

	t.Log("Verify the documented TSIG-ALLOW-AXFR zone metadata")
	var metadata struct {
		Zone     string   `json:"zone"`
		Kind     string   `json:"kind"`
		Metadata []string `json:"metadata"`
	}
	terraformOutput(t, "powerdns_zone_metadata_tsig_transfer", &metadata)
	if metadata.Zone != "tsig.example.com." || metadata.Kind != "TSIG-ALLOW-AXFR" || !slices.Equal(metadata.Metadata, []string{live["generated"].Name}) {
		t.Fatal("unexpected TSIG transfer metadata in Terraform state")
	}
	var entries []zoneMetadata
	doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/tsig.example.com./metadata"), &entries)
	byKind := map[string][]string{}
	for _, entry := range entries {
		byKind[entry.Kind] = entry.Metadata
	}
	assertMetadataKindValues(t, byKind, "TSIG-ALLOW-AXFR", []string{live["generated"].Name})

	t.Log("Import a generated key without rotating its secret")
	runTerraform(t, "state", "rm", "powerdns_tsigkey.generated")
	runTerraform(t, "import", "-input=false", "-no-color", "powerdns_tsigkey.generated", live["generated"].ID)
	apply(map[string][]string{})
	checkTSIGFixtures(t, want)
	stablePlan()

	t.Log("Changing only a write-only input at the same version must leave an empty plan")
	flags = append(flags, "-var=tsig_wo_secret_revision=2")
	stablePlan()
	checkTSIGFixtures(t, want)

	t.Log("Rotate an explicit secret with an in-place update")
	flags = append(flags, "-var=tsig_explicit_rotated=true")
	apply(map[string][]string{"powerdns_tsigkey.explicit": {"update"}})
	explicit := want["explicit"]
	explicit.Secret = secret("e2e-tsig-explicit-rotated-secret")
	want["explicit"] = explicit
	checkTSIGFixtures(t, want)
	stablePlan()

	t.Log("A rename replaces the key and updates dependent zone references in the same apply")
	oldID := live["write_only"].ID
	flags = append(flags, "-var=tsig_wo_name=e2e-tsig-renamed.")
	apply(map[string][]string{
		"powerdns_tsigkey.write_only":   {"delete", "create"},
		"powerdns_zone.tsig_write_only": {"update"},
	})
	writeOnly := want["write_only"]
	writeOnly.Name = "e2e-tsig-renamed."
	writeOnly.Secret = secret("e2e-tsig-write-only-replacement-secret")
	want["write_only"] = writeOnly
	checkTSIGFixtures(t, want)
	checkWriteOnlyZone("e2e-tsig-renamed.")
	assertTSIGKeyDeleted(t, oldID, writeOnly.Algorithm)
	stablePlan()

	t.Log("Rotate the write-only secret only when its version increases")
	flags = append(flags, "-var=tsig_wo_version=2", "-var=tsig_wo_secret_revision=3")
	apply(map[string][]string{"powerdns_tsigkey.write_only": {"update"}})
	writeOnly.Secret = secret("e2e-tsig-write-only-rotated-secret")
	writeOnly.WriteOnlyVersion = 2
	want["write_only"] = writeOnly
	checkTSIGFixtures(t, want)
	stablePlan()

	t.Log("Migrate generated and explicit keys to write-only mode and clear their persisted secrets")
	flags = append(flags, "-var=tsig_migrate_to_write_only=true")
	apply(map[string][]string{"powerdns_tsigkey.generated": {"update"}, "powerdns_tsigkey.explicit": {"update"}})
	generated.Secret = secret("e2e-tsig-generated-migration-secret")
	generated.WriteOnlyVersion = 1
	explicit.WriteOnlyVersion = 1
	want["generated"], want["explicit"] = generated, explicit
	checkTSIGFixtures(t, want)
	stablePlan()

	t.Log("Replace a key on an algorithm change without leaving a duplicate algorithm behind")
	flags = append(flags, "-var=tsig_algorithm=hmac-sha512")
	apply(map[string][]string{"powerdns_tsigkey.explicit": {"delete", "create"}})
	explicit.Algorithm = "hmac-sha512"
	want["explicit"] = explicit
	live = checkTSIGFixtures(t, want)
	stablePlan()

	t.Log("Delete all managed TSIG keys while preserving the zone")
	args := append([]string{"destroy", "-input=false", "-no-color", "-auto-approve"}, targets...)
	runTerraform(t, append(args, flags...)...)
	for _, key := range live {
		assertTSIGKeyDeleted(t, key.ID, key.Algorithm)
	}
	var zone authZone
	doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/tsig.example.com."), &zone)
	if zone.Name != "tsig.example.com." || zone.Kind != "Master" {
		t.Fatal("deleting TSIG keys changed the associated zone")
	}
}

func TestPowerDNSTSIGAlgorithmScope(t *testing.T) {
	const address = "powerdns_tsigkey.algorithm_scope"
	const id = "e2e-tsig-algorithm-scope."
	const peerAlgorithm = "hmac-sha384"
	basePath := "/api/v1/servers/localhost/tsigkeys/" + id

	// Both keys under this dedicated fixture name belong to this test. Older
	// servers only support deleting by name, so clean them up together.
	t.Cleanup(func() {
		resp, err := http.DefaultClient.Do(newRequest(t, http.MethodDelete, authBaseURL(), basePath))
		if err != nil {
			t.Errorf("cleaning up algorithm-scope TSIG keys: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
			t.Errorf("cleaning up algorithm-scope TSIG keys returned HTTP %d", resp.StatusCode)
		}
	})
	t.Log("Import the managed key before adding another algorithm with the same name")
	runTerraform(t, "state", "rm", address)
	runTerraform(t, "import", "-input=false", "-no-color", address, id)

	peerSecret := base64.StdEncoding.EncodeToString([]byte("e2e-tsig-unmanaged-secret"))
	payload, err := json.Marshal(map[string]string{"algorithm": peerAlgorithm, "key": peerSecret})
	if err != nil {
		t.Fatal(err)
	}
	// PUT can add an algorithm even on older servers whose POST rejects a
	// name that already exists. The original algorithm remains in the backend.
	request := newRequest(t, http.MethodPut, authBaseURL(), tsigKeyPath(id, "hmac-sha256"))
	request.Body = io.NopCloser(strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	var peer tsigKeyValues
	doJSON(t, request, &peer)
	if peer.Algorithm != peerAlgorithm || peer.Key != peerSecret {
		t.Fatal("failed to create the second TSIG algorithm")
	}
	assertAlgorithms := func(want []string) {
		t.Helper()
		var keys []tsigKeyValues
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/tsigkeys"), &keys)
		var got []string
		for _, key := range keys {
			if key.ID == id {
				got = append(got, key.Algorithm)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("TSIG algorithms: got %v, want %v", got, want)
		}
	}
	assertAlgorithms([]string{"hmac-sha256", peerAlgorithm})

	// Servers before algorithm-filter support silently ignore this parameter.
	resp, err := http.DefaultClient.Do(newRequest(t, http.MethodGet, authBaseURL(), tsigKeyPath(id, "terraform-provider-powerdns-probe")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	filtering := resp.StatusCode == http.StatusNotFound
	if !filtering && resp.StatusCode != http.StatusOK {
		t.Fatalf("checking TSIG algorithm filtering returned HTTP %d", resp.StatusCode)
	}
	args := []string{"apply", "-input=false", "-no-color", "-auto-approve", "-target=" + address, "-var=tsig_scoped_algorithm=hmac-sha512"}
	if filtering {
		t.Log("Algorithm replacement and destroy must preserve the unrelated algorithm")
		runTerraform(t, args...)
		assertAlgorithms([]string{peerAlgorithm, "hmac-sha512"})
		runTerraform(t, "destroy", "-input=false", "-no-color", "-auto-approve", "-target="+address, "-var=tsig_scoped_algorithm=hmac-sha512")
		assertAlgorithms([]string{peerAlgorithm})
		var remaining tsigKeyValues
		doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), tsigKeyPath(id, peerAlgorithm)), &remaining)
		if remaining.Key != peerSecret {
			t.Fatal("deleting the managed algorithm changed the unrelated secret")
		}
	} else {
		t.Log("A server that ignores algorithm filters must reject unsafe replacement and destroy")
		for _, command := range [][]string{args, {"destroy", "-input=false", "-no-color", "-auto-approve", "-target=" + address}} {
			output, err := exec.Command("terraform", command...).CombinedOutput()
			if err == nil || !strings.Contains(string(output), "algorithm filtering") {
				t.Fatalf("terraform %s must reject deleting same-name keys without algorithm filtering", command[0])
			}
			assertAlgorithms([]string{"hmac-sha256", peerAlgorithm})
		}
	}
}
