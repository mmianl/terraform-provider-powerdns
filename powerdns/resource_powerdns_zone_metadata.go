package powerdns

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePDNSZoneMetadata() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSZoneMetadataCreate,
		ReadContext:   resourcePDNSZoneMetadataRead,
		UpdateContext: resourcePDNSZoneMetadataUpdate,
		DeleteContext: resourcePDNSZoneMetadataDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSZoneMetadataImport,
		},

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "Zone name, for example \"example.com.\".",
			},
			"kind": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Metadata kind as used by PowerDNS, for example \"ALSO-NOTIFY\" or \"ALLOW-AXFR-FROM\".",
			},
			"metadata": {
				Type:        schema.TypeSet,
				Required:    true,
				MinItems:    1,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Set of values for this metadata kind.",
			},
		},
	}
}

func resourcePDNSZoneMetadataCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	kind := d.Get("kind").(string)
	values := expandStringSet(d.Get("metadata").(*schema.Set))

	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "kind", kind)
	tflog.Debug(ctx, "Creating PowerDNS zone metadata")

	if err := client.PDNS.ReplaceZoneMetadata(ctx, zone, kind, values); err != nil {
		return diag.FromErr(fmt.Errorf("error creating zone metadata %s for %s: %w", kind, zone, err))
	}

	d.SetId(zoneMetadataID(zone, kind))
	return resourcePDNSZoneMetadataRead(ctx, d, meta)
}

func resourcePDNSZoneMetadataRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	kind := d.Get("kind").(string)

	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "kind", kind)
	tflog.Debug(ctx, "Reading PowerDNS zone metadata")

	allMetadata, err := client.PDNS.ListZoneMetadata(ctx, zone)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch zone metadata for %s: %w", zone, err))
	}

	values := getMetadataValues(allMetadata, kind)
	if len(values) == 0 {
		tflog.Warn(ctx, "Zone metadata not found; removing from state")
		d.SetId("")
		return nil
	}

	if err := d.Set("zone", zone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone for metadata resource: %w", err))
	}
	if err := d.Set("kind", kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting kind for metadata resource: %w", err))
	}
	if err := d.Set("metadata", values); err != nil {
		return diag.FromErr(fmt.Errorf("error setting metadata values: %w", err))
	}

	return nil
}

func resourcePDNSZoneMetadataUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	kind := d.Get("kind").(string)

	if d.HasChange("metadata") {
		values := expandStringSet(d.Get("metadata").(*schema.Set))
		if err := client.PDNS.ReplaceZoneMetadata(ctx, zone, kind, values); err != nil {
			return diag.FromErr(fmt.Errorf("error updating zone metadata %s for %s: %w", kind, zone, err))
		}
	}

	return resourcePDNSZoneMetadataRead(ctx, d, meta)
}

func resourcePDNSZoneMetadataDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	kind := d.Get("kind").(string)

	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "kind", kind)
	tflog.Debug(ctx, "Deleting PowerDNS zone metadata")

	if err := client.PDNS.DeleteZoneMetadata(ctx, zone, kind); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting zone metadata %s for %s: %w", kind, zone, err))
	}

	return nil
}

func resourcePDNSZoneMetadataImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	zone, kind, err := parseZoneMetadataID(d.Id())
	if err != nil {
		return nil, err
	}

	if err := d.Set("zone", zone); err != nil {
		return nil, fmt.Errorf("error setting zone in import: %w", err)
	}
	if err := d.Set("kind", kind); err != nil {
		return nil, fmt.Errorf("error setting kind in import: %w", err)
	}
	d.SetId(zoneMetadataID(zone, kind))

	return []*schema.ResourceData{d}, nil
}

func zoneMetadataID(zone string, kind string) string {
	return zone + idSeparator + kind
}

func parseZoneMetadataID(id string) (string, string, error) {
	parts := strings.Split(id, idSeparator)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid zone metadata id %q, expected <zone>%s<kind>", id, idSeparator)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid zone metadata id %q, zone and kind must be non-empty", id)
	}
	return parts[0], parts[1], nil
}
