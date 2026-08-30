package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSZone() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSZoneCreate,
		ReadContext:   resourcePDNSZoneRead,
		UpdateContext: resourcePDNSZoneUpdate,
		DeleteContext: resourcePDNSZoneDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateZoneName,
			},

			"kind": {
				Type:     schema.TypeString,
				Required: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.EqualFold(old, new)
				},
			},

			"account": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "admin",
				ForceNew:     false,
				ValidateFunc: validation.StringLenBetween(0, 40),
			},

			"catalog": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     false,
				ValidateFunc: ValidateZoneName,
			},

			"masters": {
				Type: schema.TypeSet,
				// TypeSet identities are computed before an element StateFunc runs.
				// Hashing the canonical form makes expanded and compressed IPv6
				// spellings represent the same set element.
				Set: func(value interface{}) int {
					return schema.HashString(NormalizeMasterAddress(value.(string)))
				},
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: ValidateMasterAddress,
					// PowerDNS returns IPv6 addresses in compressed form. Normalizing
					// the configured value prevents a perpetual diff when users write
					// an equivalent expanded address.
					StateFunc: func(value interface{}) string {
						return NormalizeMasterAddress(value.(string))
					},
				},
				Optional: true,
			},

			"soa_edit_api": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: false,
			},
		},
	}
}

func resourcePDNSZoneCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	// Each element is validated by the schema's ValidateFunc, which runs on
	// create and on update alike.
	var masters []string
	for _, masterAddress := range d.Get("masters").(*schema.Set).List() {
		masters = append(masters, masterAddress.(string))
	}

	zoneInfo := ZoneInfo{
		Name:        d.Get("name").(string),
		Kind:        d.Get("kind").(string),
		Catalog:     d.Get("catalog").(string),
		Account:     d.Get("account").(string),
		Nameservers: []string{},
		SoaEditAPI:  d.Get("soa_edit_api").(string),
	}

	if len(masters) != 0 {
		if strings.EqualFold(zoneInfo.Kind, "Slave") {
			zoneInfo.Masters = masters
		} else {
			return diag.FromErr(fmt.Errorf("masters attribute is supported only for Slave kind"))
		}
	}

	tflog.SetField(ctx, "zone_name", zoneInfo.Name)
	tflog.SetField(ctx, "zone_kind", zoneInfo.Kind)
	tflog.Debug(ctx, "Creating PowerDNS Zone")

	createdZoneInfo, err := client.PDNS.CreateZone(ctx, zoneInfo)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(createdZoneInfo.ID)
	tflog.Info(ctx, "Created PowerDNS Zone", map[string]any{"id": createdZoneInfo.ID})
	return resourcePDNSZoneRead(ctx, d, meta)
}

func resourcePDNSZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Reading PowerDNS Zone")

	zoneInfo, err := client.PDNS.GetZone(ctx, d.Id())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing from state")
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS Zone: %w", err))
	}

	if zoneInfo.Name == "" {
		tflog.Warn(ctx, "Zone not found; removing from state")
		d.SetId("")
		return nil
	}

	if err := d.Set("name", zoneInfo.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Name: %w", err))
	}
	if err := d.Set("kind", zoneInfo.Kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Kind: %w", err))
	}
	if err := d.Set("account", zoneInfo.Account); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Account: %w", err))
	}
	if err := d.Set("catalog", zoneInfo.Catalog); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Catalog: %w", err))
	}
	if err := d.Set("soa_edit_api", zoneInfo.SoaEditAPI); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS SOA Edit API: %w", err))
	}

	if strings.EqualFold(zoneInfo.Kind, "Slave") {
		if err := d.Set("masters", zoneInfo.Masters); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS Masters: %w", err))
		}
	}

	return nil
}

func resourcePDNSZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Updating PowerDNS Zone")

	client := meta.(*ProviderClients)

	if d.HasChange("kind") || d.HasChange("account") || d.HasChange("catalog") || d.HasChange("soa_edit_api") || d.HasChange("masters") {
		var masters []string
		for _, m := range d.Get("masters").(*schema.Set).List() {
			masters = append(masters, m.(string))
		}

		zoneInfo := ZoneInfoUpd{
			Name:       d.Get("name").(string),
			Kind:       d.Get("kind").(string),
			Catalog:    d.Get("catalog").(string),
			Account:    d.Get("account").(string),
			SoaEditAPI: d.Get("soa_edit_api").(string),
			Masters:    masters,
		}

		if err := client.PDNS.UpdateZone(ctx, d.Id(), zoneInfo); err != nil {
			return diag.FromErr(fmt.Errorf("error updating PowerDNS Zone: %w", err))
		}
	}

	return resourcePDNSZoneRead(ctx, d, meta)
}

func resourcePDNSZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Deleting PowerDNS Zone")

	if err := client.PDNS.DeleteZone(ctx, d.Id()); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS Zone: %w", err))
	}
	tflog.Info(ctx, "Deleted PowerDNS Zone")
	return nil
}
