package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSReverseZone() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSReverseZoneCreate,
		ReadContext:   resourcePDNSReverseZoneRead,
		UpdateContext: resourcePDNSReverseZoneUpdate,
		DeleteContext: resourcePDNSReverseZoneDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSReverseZoneImport,
		},

		Schema: map[string]*schema.Schema{
			"cidr": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateCIDR,
				Description:  "The CIDR block for the reverse zone (e.g., '172.16.0.0/16')",
			},
			"kind": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"Master", "Slave"}, false),
				Description:  "The kind of zone (Master or Slave)",
			},
			"nameservers": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringLenBetween(1, 255),
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

func expandStringList(configured []interface{}) []string {
	vs := make([]string, 0, len(configured))
	for _, v := range configured {
		vs = append(vs, v.(string))
	}
	return vs
}

func resourcePDNSReverseZoneCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	cidr := d.Get("cidr").(string)
	tflog.SetField(ctx, "cidr", cidr)
	tflog.Debug(ctx, "Creating reverse zone")

	zoneName, err := GetReverseZoneName(cidr)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to determine zone name: %w", err))
	}
	tflog.Info(ctx, "Generated reverse zone name", map[string]any{"zone": zoneName})

	zone := ZoneInfo{
		Name:        zoneName,
		Kind:        d.Get("kind").(string),
		Nameservers: expandStringList(d.Get("nameservers").([]interface{})),
	}

	createdZone, err := client.CreateZone(ctx, zone)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create reverse zone: %w", err))
	}

	d.SetId(createdZone.Name)
	tflog.Info(ctx, "Created reverse zone", map[string]any{"id": createdZone.Name})
	return resourcePDNSReverseZoneRead(ctx, d, meta)
}

func resourcePDNSReverseZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)
	zoneName := d.Id()

	tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Reading reverse zone")

	zone, err := client.GetZone(ctx, zoneName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch zone: %w", err))
	}

	// If zone doesn't exist, clear state
	if zone.Name == "" {
		tflog.Warn(ctx, "Zone not found; removing from state")
		d.SetId("")
		return nil
	}

	tflog.Info(ctx, "Found reverse zone", map[string]any{"zone": zone.Name, "kind": zone.Kind})

	if err := d.Set("name", zone.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting name: %w", err))
	}
	if err := d.Set("kind", zone.Kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting kind: %w", err))
	}

	// Read nameservers from NS records
	nameservers, err := client.ListRecordsInRRSet(ctx, zoneName, zoneName, "NS")
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

func resourcePDNSReverseZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)
	zoneName := d.Id()

	tflog.SetField(ctx, "zone", zoneName)
	if d.HasChange("nameservers") {
		tflog.Debug(ctx, "Updating nameservers for reverse zone")

		zone, err := client.GetZone(ctx, zoneName)
		if err != nil {
			return diag.FromErr(fmt.Errorf("couldn't fetch zone: %w", err))
		}

		// Update nameservers in zone object
		zone.Nameservers = expandStringList(d.Get("nameservers").([]interface{}))

		// Build update request
		zoneInfo := ZoneInfoUpd{
			Name:       zoneName,
			Kind:       zone.Kind,
			Account:    zone.Account,
			SoaEditAPI: zone.SoaEditAPI,
		}

		if err := client.UpdateZone(ctx, zoneName, zoneInfo); err != nil {
			return diag.FromErr(fmt.Errorf("error updating zone: %w", err))
		}

		// Update NS records to reflect nameserver list
		rrSet := ResourceRecordSet{
			Name:       zoneName,
			Type:       "NS",
			TTL:        3600,
			ChangeType: "REPLACE",
			Records:    make([]Record, len(zone.Nameservers)),
		}

		for i, ns := range zone.Nameservers {
			rrSet.Records[i] = Record{
				Content: ns,
				TTL:     3600,
			}
		}

		if _, err := client.ReplaceRecordSet(ctx, zoneName, rrSet); err != nil {
			return diag.FromErr(fmt.Errorf("error updating nameserver records: %w", err))
		}

		tflog.Info(ctx, "Updated nameservers for reverse zone")
	}

	return resourcePDNSReverseZoneRead(ctx, d, meta)
}

func resourcePDNSReverseZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)
	zoneName := d.Id()

	tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Deleting reverse zone")

	if err := client.DeleteZone(ctx, zoneName); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting zone: %w", err))
	}

	tflog.Info(ctx, "Deleted reverse zone")
	return nil
}

func resourcePDNSReverseZoneImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*Client)

	zoneName := d.Id()
	tflog.Info(ctx, "Importing reverse zone", map[string]any{"zone": zoneName})

	cidr, err := ParseReverseZoneName(zoneName)
	if err != nil {
		return nil, err
	}

	zone, err := client.GetZone(ctx, zoneName)
	if err != nil {
		return nil, fmt.Errorf("error getting zone: %w", err)
	}

	// Populate attributes
	if err := d.Set("name", zoneName); err != nil {
		return nil, fmt.Errorf("error setting name: %w", err)
	}
	if err := d.Set("cidr", cidr); err != nil {
		return nil, fmt.Errorf("error setting cidr: %w", err)
	}
	if err := d.Set("nameservers", zone.Nameservers); err != nil {
		return nil, fmt.Errorf("error setting nameservers: %w", err)
	}
	// Ensure kind is set to avoid diffs after import
	if err := d.Set("kind", zone.Kind); err != nil {
		return nil, fmt.Errorf("error setting kind: %w", err)
	}

	return []*schema.ResourceData{d}, nil
}
