package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSZoneMetadataList() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSZoneMetadataListRead,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "Zone name, as FQDN with trailing dot.",
			},
			"entries": {
				Type:        schema.TypeSet,
				Computed:    true,
				Description: "All metadata entries for the zone.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"kind": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Metadata kind name.",
						},
						"metadata": {
							Type:        schema.TypeSet,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Values for the metadata kind.",
						},
					},
				},
			},
		},
	}
}

func dataSourcePDNSZoneMetadataListRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	zone := d.Get("zone").(string)

	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Info(ctx, "Reading all zone metadata data source")

	allMetadata, err := client.PDNS.ListZoneMetadata(ctx, zone)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch metadata for zone %s: %w", zone, err))
	}

	entries := make([]map[string]interface{}, 0, len(allMetadata))
	for _, md := range allMetadata {
		entries = append(entries, map[string]interface{}{
			"kind":     md.Kind,
			"metadata": md.Metadata,
		})
	}

	d.SetId(zone)

	if err := d.Set("zone", zone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone: %w", err))
	}
	if err := d.Set("entries", entries); err != nil {
		return diag.FromErr(fmt.Errorf("error setting metadata entries: %w", err))
	}

	return nil
}
