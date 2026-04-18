package powerdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourcePDNSZoneMetadata_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSZoneMetadataConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_zone_metadata.also_notify", "zone", "metadata-ds.sysa.abc."),
					resource.TestCheckResourceAttr("data.powerdns_zone_metadata.also_notify", "kind", "ALSO-NOTIFY"),
					resource.TestCheckTypeSetElemAttr("data.powerdns_zone_metadata.also_notify", "metadata.*", "192.0.2.10"),
					resource.TestCheckTypeSetElemAttr("data.powerdns_zone_metadata.also_notify", "metadata.*", "192.0.2.11:5300"),
				),
			},
		},
	})
}

func TestAccDataSourcePDNSZoneMetadataList_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcePDNSZoneMetadataConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.powerdns_zone_metadata_list.all", "zone", "metadata-ds.sysa.abc."),
					resource.TestCheckResourceAttrSet("data.powerdns_zone_metadata_list.all", "entries.#"),
					resource.TestCheckTypeSetElemNestedAttrs("data.powerdns_zone_metadata_list.all", "entries.*", map[string]string{
						"kind": "ALSO-NOTIFY",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.powerdns_zone_metadata_list.all", "entries.*", map[string]string{
						"kind": "ALLOW-AXFR-FROM",
					}),
				),
			},
		},
	})
}

const testAccDataSourcePDNSZoneMetadataConfig = `
resource "powerdns_zone" "test" {
  name = "metadata-ds.sysa.abc."
  kind = "Master"
}

resource "powerdns_zone_metadata" "also_notify" {
  zone = powerdns_zone.test.name
  kind = "ALSO-NOTIFY"
  metadata = ["192.0.2.10", "192.0.2.11:5300"]
}

resource "powerdns_zone_metadata" "allow_axfr_from" {
  zone = powerdns_zone.test.name
  kind = "ALLOW-AXFR-FROM"
  metadata = ["AUTO-NS", "198.51.100.0/24"]
}

data "powerdns_zone_metadata" "also_notify" {
  zone = powerdns_zone.test.name
  kind = "ALSO-NOTIFY"
  depends_on = [powerdns_zone_metadata.also_notify, powerdns_zone_metadata.allow_axfr_from]
}

data "powerdns_zone_metadata_list" "all" {
  zone = powerdns_zone.test.name
  depends_on = [powerdns_zone_metadata.also_notify, powerdns_zone_metadata.allow_axfr_from]
}
`
