package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"slices"
	"testing"
	"time"
)

type cryptokeyDetails struct {
	KeyType   string   `json:"keytype"`
	Algorithm string   `json:"algorithm"`
	Bits      int      `json:"bits"`
	Active    bool     `json:"active"`
	Published bool     `json:"published"`
	DNSKey    string   `json:"dnskey"`
	DS        []string `json:"ds"`
	CDS       []string `json:"cds"`
}

type authCryptokey struct {
	cryptokeyDetails
	ID int `json:"id"`
}

type cryptokeyOutput struct {
	cryptokeyDetails
	ID    string `json:"id"`
	Zone  string `json:"zone"`
	KeyID int    `json:"key_id"`
}

func runTerraform(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "terraform", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("terraform %v failed: %v\n%s", args, err, output)
	}
}

func assertCryptokeyDetails(t *testing.T, got, want cryptokeyDetails) {
	t.Helper()
	if got.KeyType != want.KeyType || got.Algorithm != want.Algorithm || got.Bits != want.Bits ||
		got.Active != want.Active || got.Published != want.Published || got.DNSKey != want.DNSKey ||
		!slices.Equal(got.DS, want.DS) || !slices.Equal(got.CDS, want.CDS) {
		t.Fatalf("cryptokey attributes differ: got %+v, want %+v", got, want)
	}
}

func assertCryptokeyResource(t *testing.T, output, zone, role, algorithm string, bits int, active, published bool) (cryptokeyOutput, authCryptokey) {
	t.Helper()
	var key cryptokeyOutput
	terraformOutput(t, output, &key)
	if key.Zone != zone || key.KeyID <= 0 || key.ID != fmt.Sprintf("%s:%d", zone, key.KeyID) {
		t.Fatalf("unexpected cryptokey identity: zone=%q id=%q key_id=%d", key.Zone, key.ID, key.KeyID)
	}
	if key.KeyType != role || key.Algorithm != algorithm || key.Bits != bits ||
		key.Active != active || key.Published != published || key.DNSKey == "" {
		t.Fatalf("unexpected cryptokey state: %+v", key)
	}
	path := fmt.Sprintf("/api/v1/servers/localhost/zones/%s/cryptokeys/%d", zone, key.KeyID)
	var live authCryptokey
	doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), path), &live)
	if live.ID != key.KeyID {
		t.Fatalf("cryptokey API ID: got %d, want %d", live.ID, key.KeyID)
	}
	// PowerDNS can normalize a KSK/ZSK to CSK or a requested CSK to KSK.
	if live.KeyType != role && live.KeyType != "csk" && !(role == "csk" && live.KeyType == "ksk") {
		t.Fatalf("cryptokey API role: got %q for requested role %q", live.KeyType, role)
	}
	expected := key.cryptokeyDetails
	expected.KeyType = live.KeyType
	assertCryptokeyDetails(t, live.cryptokeyDetails, expected)
	return key, live
}

func assertCryptokeyDataSource(t *testing.T, liveKeys ...authCryptokey) {
	t.Helper()
	var output struct {
		Zone       string            `json:"zone"`
		Cryptokeys []cryptokeyOutput `json:"cryptokeys"`
	}
	terraformOutput(t, "data_powerdns_zone_cryptokeys_test", &output)
	if output.Zone != "cryptokey.example.com." || len(output.Cryptokeys) != len(liveKeys) {
		t.Fatalf("cryptokey data source: zone=%q key count=%d", output.Zone, len(output.Cryptokeys))
	}
	expected := make(map[int]authCryptokey, len(liveKeys))
	for _, live := range liveKeys {
		expected[live.ID] = live
	}
	for _, key := range output.Cryptokeys {
		live, ok := expected[key.KeyID]
		if !ok {
			t.Fatalf("cryptokey data source returned an unexpected or duplicate key ID: %d", key.KeyID)
		}
		// The data source exposes server roles, including CSK normalization.
		assertCryptokeyDetails(t, key.cryptokeyDetails, live.cryptokeyDetails)
		delete(expected, key.KeyID)
	}
}

func assertCryptokeyDeleted(t *testing.T, key cryptokeyOutput) {
	t.Helper()
	path := fmt.Sprintf("/api/v1/servers/localhost/zones/%s/cryptokeys/%d", key.Zone, key.KeyID)
	resp, err := http.DefaultClient.Do(newRequest(t, http.MethodGet, authBaseURL(), path))
	if err != nil {
		t.Fatalf("checking deleted cryptokey: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted cryptokey returned HTTP %d, want 404", resp.StatusCode)
	}
}

func TestPowerDNSCryptokeys(t *testing.T) {
	const resource = "powerdns_zone_cryptokey.test"
	const zone = "cryptokey.example.com."
	check := func(role string, active, published bool) cryptokeyOutput {
		t.Helper()
		key, live := assertCryptokeyResource(t, "powerdns_zone_cryptokey_test", zone, role, "ECDSAP256SHA256", 256, active, published)
		assertCryptokeyDataSource(t, live)
		return key
	}
	// Keep lifecycle checks scoped to these resources: the shared fixture's
	// Recursor hostname is resolved to an IP by PowerDNS and has unrelated drift.
	targets := []string{
		"-target=" + resource,
		"-target=powerdns_zone_cryptokey.rsa",
		"-target=powerdns_zone_cryptokey.split_zsk",
		"-target=data.powerdns_zone_cryptokeys.test",
	}
	plan := func(vars ...string) {
		t.Helper()
		// Exit code 2 from -detailed-exitcode fails the test if any drift remains.
		args := append([]string{"plan", "-input=false", "-no-color", "-detailed-exitcode"}, targets...)
		runTerraform(t, append(args, vars...)...)
	}
	apply := func(vars ...string) {
		t.Helper()
		args := append([]string{"apply", "-input=false", "-no-color", "-auto-approve"}, targets...)
		runTerraform(t, append(args, vars...)...)
	}

	t.Log("Verify generated ECDSA and RSA keys, their public attributes, and an unchanged plan")
	key := check("csk", true, true)
	rsa, _ := assertCryptokeyResource(t, "powerdns_zone_cryptokey_rsa", "cryptokey-rsa.example.com.", "ksk", "RSASHA256", 2048, true, true)
	if len(key.DS) == 0 || len(rsa.DS) == 0 {
		t.Fatal("generated signing keys must expose DS records")
	}
	plan()

	t.Log("Preserve the CSK and its DS records when an active ZSK makes PowerDNS report it as KSK")
	assertOriginalKey := func(current cryptokeyOutput) {
		t.Helper()
		if current.ID != key.ID || current.DNSKey != key.DNSKey || !slices.Equal(current.DS, key.DS) {
			t.Fatal("split signing replaced the CSK or changed its DS records")
		}
	}
	split := []string{"-var=cryptokey_split_signing=true"}
	apply(split...)
	combined, reported := assertCryptokeyResource(t, "powerdns_zone_cryptokey_test", zone, "csk", "ECDSAP256SHA256", 256, true, true)
	if reported.KeyType != "ksk" {
		t.Fatalf("split-signing regression was not exercised: PowerDNS reported %q, want ksk", reported.KeyType)
	}
	assertOriginalKey(combined)
	var splitKeys []authCryptokey
	doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/"+zone+"/cryptokeys"), &splitKeys)
	if len(splitKeys) != 2 {
		t.Fatalf("split signing requires two keys, got %d", len(splitKeys))
	}
	for _, live := range splitKeys {
		if live.ID != key.KeyID && (live.KeyType != "zsk" || !live.Active || live.Algorithm != key.Algorithm) {
			t.Fatalf("expected an active ZSK of the same algorithm, got %+v", live)
		}
	}
	assertCryptokeyDataSource(t, splitKeys...)
	plan(split...)
	apply(split...)
	combined, _ = assertCryptokeyResource(t, "powerdns_zone_cryptokey_test", zone, "csk", "ECDSAP256SHA256", 256, true, true)
	assertOriginalKey(combined)
	plan(split...)
	apply()
	assertOriginalKey(check("csk", true, true))
	plan()

	t.Log("Update active and published flags without replacing key material")
	flags := []string{"-var=cryptokey_active=false", "-var=cryptokey_published=false"}
	apply(flags...)
	updated := check("csk", false, false)
	if updated.ID != key.ID || updated.DNSKey != key.DNSKey {
		t.Fatal("changing active/published flags replaced the key")
	}
	plan(flags...)

	t.Log("Replace the key on role changes and verify subsequent plans are empty")
	for _, role := range []string{"ksk", "zsk", "csk"} {
		vars := []string{"-var=cryptokey_keytype=" + role}
		apply(vars...)
		replacement := check(role, true, true)
		if replacement.DNSKey == key.DNSKey {
			t.Fatalf("changing the role to %s did not replace key material", role)
		}
		if replacement.KeyID != key.KeyID {
			assertCryptokeyDeleted(t, key)
		}
		key = replacement
		plan(vars...)
	}

	t.Log("Import an existing key without replacing its material")
	runTerraform(t, "state", "rm", resource)
	runTerraform(t, "import", "-input=false", "-no-color", resource, key.ID)
	apply()
	imported := check("csk", true, true)
	if imported.ID != key.ID || imported.DNSKey != key.DNSKey {
		t.Fatal("import replaced the existing key")
	}
	plan()

	t.Log("Delete only the managed key while preserving its zone and the RSA key")
	runTerraform(t, "destroy", "-input=false", "-no-color", "-auto-approve", "-target="+resource)
	assertCryptokeyDeleted(t, key)
	var remaining []authCryptokey
	doJSON(t, newRequest(t, http.MethodGet, authBaseURL(), "/api/v1/servers/localhost/zones/"+zone+"/cryptokeys"), &remaining)
	if len(remaining) != 0 {
		t.Fatalf("managed key was not removed: %d keys remain", len(remaining))
	}
	assertCryptokeyResource(t, "powerdns_zone_cryptokey_rsa", rsa.Zone, "ksk", "RSASHA256", 2048, true, true)
}
