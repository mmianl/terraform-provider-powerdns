package powerdns

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSZone() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSZoneRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the zone to retrieve",
			},
			"kind": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The kind of zone (Master, Slave, etc.)",
			},
			"account": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The account associated with the zone",
			},
			"nameservers": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of nameservers for this zone",
			},
			"masters": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of master servers for this zone (Slave zones only)",
			},
			"soa_edit_api": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "SOA edit API setting",
			},
			"records": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the record",
						},
						"type": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The type of the record (A, AAAA, CNAME, etc.)",
						},
						"content": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The content of the record",
						},
						"ttl": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The TTL of the record",
						},
						"disabled": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the record is disabled",
						},
					},
				},
				Description: "List of all records in the zone",
			},
		},
	}
}

func dataSourcePDNSZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zoneName := d.Get("name").(string)
	ctx = tflog.SetField(ctx, "zone_name", zoneName)
	tflog.Info(ctx, "Reading zone data source")

	// Get the zone information
	zone, err := client.PDNS.GetZone(ctx, zoneName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch zone %s: %w", zoneName, err))
	}

	// Check if zone exists
	if zone.Name == "" {
		return diag.FromErr(fmt.Errorf("zone %s not found", zoneName))
	}

	ctx = tflog.SetField(ctx, "kind", zone.Kind)
	tflog.Info(ctx, "Found zone")

	// Set zone information
	d.SetId(zone.Name)

	if err := d.Set("name", zone.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone name: %w", err))
	}
	if err := d.Set("kind", zone.Kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone kind: %w", err))
	}
	if err := d.Set("account", zone.Account); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone account: %w", err))
	}
	if err := d.Set("soa_edit_api", zone.SoaEditAPI); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone SOA edit API: %w", err))
	}

	// Set nameservers for non-Slave zones
	if !strings.EqualFold(zone.Kind, "Slave") {
		nameservers, err := client.PDNS.ListRecordsInRRSet(ctx, zoneName, zoneName, "NS")
		if err != nil {
			return diag.FromErr(fmt.Errorf("couldn't fetch zone %s nameservers from PowerDNS: %w", zoneName, err))
		}

		var zoneNameservers []string
		for _, ns := range nameservers {
			zoneNameservers = append(zoneNameservers, ns.Content)
		}

		if err := d.Set("nameservers", zoneNameservers); err != nil {
			return diag.FromErr(fmt.Errorf("error setting zone nameservers: %w", err))
		}
	}

	// Set masters for Slave zones
	if strings.EqualFold(zone.Kind, "Slave") {
		if err := d.Set("masters", zone.Masters); err != nil {
			return diag.FromErr(fmt.Errorf("error setting zone masters: %w", err))
		}
	}

	// Get all records in the zone and link them to the zone data
	allRecords, err := client.PDNS.ListRecords(ctx, zoneName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch records for zone %s: %w", zoneName, err))
	}

	// Convert records to the schema format
	records := make([]map[string]interface{}, 0, len(allRecords))
	for _, r := range allRecords {
		records = append(records, map[string]interface{}{
			"name":     r.Name,
			"type":     r.Type,
			"content":  r.Content,
			"ttl":      r.TTL,
			"disabled": r.Disabled,
		})
	}

	if err := d.Set("records", records); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone records: %w", err))
	}

	tflog.Info(ctx, "Successfully retrieved zone records", map[string]interface{}{
		"record_count": len(records),
	})
	return nil
}
