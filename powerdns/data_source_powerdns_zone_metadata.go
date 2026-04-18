package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSZoneMetadata() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSZoneMetadataRead,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "Zone name, as FQDN with trailing dot.",
			},
			"kind": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Metadata kind name (for example ALSO-NOTIFY).",
			},
			"metadata": {
				Type:        schema.TypeSet,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Values for the requested metadata kind.",
			},
		},
	}
}

func dataSourcePDNSZoneMetadataRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	kind := d.Get("kind").(string)

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "kind", kind)
	tflog.Info(ctx, "Reading zone metadata data source")

	md, err := client.PDNS.GetZoneMetadata(ctx, zone, kind)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch metadata %s for zone %s: %w", kind, zone, err))
	}

	values := append([]string(nil), md.Metadata...)
	d.SetId(zoneMetadataID(zone, kind))

	if err := d.Set("zone", zone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone: %w", err))
	}
	if err := d.Set("kind", kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting kind: %w", err))
	}
	if err := d.Set("metadata", values); err != nil {
		return diag.FromErr(fmt.Errorf("error setting metadata: %w", err))
	}

	return nil
}
