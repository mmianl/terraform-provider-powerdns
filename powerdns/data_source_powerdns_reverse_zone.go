package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSReverseZone() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSReverseZoneRead,

		Schema: map[string]*schema.Schema{
			"cidr": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateCIDR,
				Description:  "The CIDR block for the reverse zone (e.g., '172.16.0.0/16')",
			},
			"kind": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The kind of zone (Master or Slave)",
			},
			"nameservers": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of nameservers for this zone",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed zone name (e.g., '16.172.in-addr.arpa.')",
			},
		},
	}
}

func dataSourcePDNSReverseZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	cidr := d.Get("cidr").(string)
	ctx = tflog.SetField(ctx, "cidr", cidr)
	tflog.Info(ctx, "Reading reverse zone data source")

	zoneName, err := GetReverseZoneName(cidr)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to determine zone name: %w", err))
	}
	ctx = tflog.SetField(ctx, "zone_name", zoneName)
	tflog.Debug(ctx, "Computed reverse zone name from CIDR")

	zone, err := client.PDNS.GetZone(ctx, zoneName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch zone: %w", err))
	}

	// Check if zone exists by checking if the name is empty
	if zone.Name == "" {
		return diag.FromErr(fmt.Errorf("reverse zone for CIDR %s not found", cidr))
	}

	tflog.Info(ctx, "Found reverse zone", map[string]interface{}{
		"name": zone.Name,
		"kind": zone.Kind,
	})

	d.SetId(zone.Name)

	if err := d.Set("name", zone.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting name: %w", err))
	}
	if err := d.Set("kind", zone.Kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting kind: %w", err))
	}

	// Read nameservers from NS records
	nameservers, err := client.PDNS.ListRecordsInRRSet(ctx, zoneName, zoneName, "NS")
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch zone %s nameservers from PowerDNS: %w", zoneName, err))
	}

	var zoneNameservers []string
	for _, ns := range nameservers {
		zoneNameservers = append(zoneNameservers, ns.Content)
	}

	if err := d.Set("nameservers", zoneNameservers); err != nil {
		return diag.FromErr(fmt.Errorf("error setting nameservers: %w", err))
	}

	return nil
}
