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
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateZoneName,
				Description:  "The name of the zone to retrieve. Must be a fully qualified domain name ending with a trailing dot or zone variant.",
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
			"catalog": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The catalog zone this zone belongs to",
			},
			"soa_edit": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "SOA edit setting applied when serving the zone",
			},
			"dnssec": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether DNSSEC is enabled for this zone",
			},
			"api_rectify": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the zone is rectified automatically after API changes",
			},
			"master_tsig_key_ids": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "TSIG keys used to sign outgoing AXFR/NOTIFY for this zone",
			},
			"slave_tsig_key_ids": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "TSIG keys used when retrieving this zone from a master",
			},
			"serial": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The current serial of the zone",
			},
			"notified_serial": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The last serial this zone was notified for",
			},
			"edited_serial": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The serial of the zone as last edited",
			},
			"last_check": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Timestamp of the last freshness check (Slave zones only)",
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
	if err := d.Set("catalog", zone.Catalog); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone catalog: %w", err))
	}
	if err := d.Set("soa_edit", zone.SoaEdit); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone SOA edit: %w", err))
	}
	if err := d.Set("dnssec", zone.DNSSec); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone DNSSEC: %w", err))
	}
	if err := d.Set("api_rectify", zone.APIRectify); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone API rectify: %w", err))
	}
	if err := d.Set("master_tsig_key_ids", zone.MasterTsigKeyIDs); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone master TSIG key IDs: %w", err))
	}
	if err := d.Set("slave_tsig_key_ids", zone.SlaveTsigKeyIDs); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone slave TSIG key IDs: %w", err))
	}
	if err := d.Set("serial", zone.Serial); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone serial: %w", err))
	}
	if err := d.Set("notified_serial", zone.NotifiedSerial); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone notified serial: %w", err))
	}
	if err := d.Set("edited_serial", zone.EditedSerial); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone edited serial: %w", err))
	}
	if err := d.Set("last_check", zone.LastCheck); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone last check: %w", err))
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
