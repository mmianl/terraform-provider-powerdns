package powerdns

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPDNSZoneMetadata_basic(t *testing.T) {
	resourceName := "powerdns_zone_metadata.test-also-notify"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneMetadataConfigBasic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "zone", "metadata-test.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "ALSO-NOTIFY"),
					resource.TestCheckTypeSetElemAttr(resourceName, "metadata.*", "192.0.2.10"),
					resource.TestCheckTypeSetElemAttr(resourceName, "metadata.*", "192.0.2.11:5300"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testPDNSZoneMetadataConfigUpdated,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "zone", "metadata-test.sysa.abc."),
					resource.TestCheckResourceAttr(resourceName, "kind", "ALSO-NOTIFY"),
					resource.TestCheckTypeSetElemAttr(resourceName, "metadata.*", "192.0.2.99"),
					resource.TestCheckResourceAttr(resourceName, "metadata.#", "1"),
				),
			},
		},
	})
}

func TestParseZoneMetadataIDInvalid(t *testing.T) {
	_, _, err := parseZoneMetadataID("invalid-format")
	if err == nil {
		t.Fatal("expected parseZoneMetadataID to return an error for invalid id format")
	}
}

const testPDNSZoneMetadataConfigBasic = `
resource "powerdns_zone" "test" {
  name = "metadata-test.sysa.abc."
  kind = "Master"
}

resource "powerdns_zone_metadata" "test-also-notify" {
  zone = powerdns_zone.test.name
  kind = "ALSO-NOTIFY"
  metadata = ["192.0.2.10", "192.0.2.11:5300"]
}
`

const testPDNSZoneMetadataConfigUpdated = `
resource "powerdns_zone" "test" {
  name = "metadata-test.sysa.abc."
  kind = "Master"
}

resource "powerdns_zone_metadata" "test-also-notify" {
  zone = powerdns_zone.test.name
  kind = "ALSO-NOTIFY"
  metadata = ["192.0.2.99"]
}
`

func TestZoneMetadataCreateKeepsStateWhenReadFails(t *testing.T) {
	client := newTestClient(writesSucceedReadsFail(http.StatusNoContent, "", 0, ""))
	assertCreateKeepsState(t, newZoneMetadataResource(), &ProviderClients{PDNS: client},
		map[string]tftypes.Value{"zone": tfString("kept.example."), "kind": tfString("ALSO-NOTIFY"), "metadata": tfStringSet("192.0.2.1")},
		"kept.example.:::ALSO-NOTIFY")
}
