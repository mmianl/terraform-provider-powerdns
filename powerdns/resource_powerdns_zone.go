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
			"soa_edit": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: false,
			},

			"dnssec": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				ForceNew: false,
			},

			"api_rectify": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				ForceNew: false,
			},

			"serial": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"notified_serial": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"edited_serial": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"last_check": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"master_tsig_key_ids": zoneTSIGReferencesSchema("TSIG key IDs allowed to authenticate AXFR requests for this zone.", 0),
			"slave_tsig_key_ids":  zoneTSIGReferencesSchema("TSIG key IDs used to sign requests to this zone's primary server.", 1),
		},
	}
}

// NormalizeTSIGKeyID accepts a bare name or the dotted ID returned by PowerDNS.
func NormalizeTSIGKeyID(keyID string) string {
	if keyID == "" || strings.HasSuffix(keyID, ".") {
		return keyID
	}
	return keyID + "."
}

func zoneTSIGReferencesSchema(description string, maxItems int) *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Computed:    true,
		MaxItems:    maxItems,
		Description: description + " Bare names are normalized to dotted IDs; set [] to clear existing references.",
		// A set computes identity before applying its element StateFunc.
		Set: func(value interface{}) int {
			return schema.HashString(NormalizeTSIGKeyID(value.(string)))
		},
		Elem: &schema.Schema{
			Type: schema.TypeString,
			StateFunc: func(value interface{}) string {
				return NormalizeTSIGKeyID(value.(string))
			},
		},
	}
}

func zoneTSIGReferences(d *schema.ResourceData, name string) []string {
	values := d.Get(name).(*schema.Set).List()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, NormalizeTSIGKeyID(value.(string)))
	}
	return result
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
		Name:             d.Get("name").(string),
		Kind:             d.Get("kind").(string),
		Catalog:          d.Get("catalog").(string),
		Account:          d.Get("account").(string),
		Nameservers:      []string{},
		SoaEdit:          d.Get("soa_edit").(string),
		SoaEditAPI:       d.Get("soa_edit_api").(string),
		DNSSec:           d.Get("dnssec").(bool),
		MasterTsigKeyIDs: zoneTSIGReferences(d, "master_tsig_key_ids"),
		SlaveTsigKeyIDs:  zoneTSIGReferences(d, "slave_tsig_key_ids"),
	}

	// Inspect configuration because an explicit false must be sent, while an
	// omitted Optional+Computed boolean must preserve the server default.
	if config := d.GetRawConfig(); !config.IsNull() {
		value := config.GetAttr("api_rectify")
		if value.IsKnown() && !value.IsNull() {
			apiRectify := d.Get("api_rectify").(bool)
			zoneInfo.APIRectify = &apiRectify
		}
	}

	if len(masters) != 0 {
		if strings.EqualFold(zoneInfo.Kind, "Slave") {
			zoneInfo.Masters = masters
		} else {
			return diag.FromErr(fmt.Errorf("masters attribute is supported only for Slave kind"))
		}
	}

	ctx = tflog.SetField(ctx, "zone_name", zoneInfo.Name)
	ctx = tflog.SetField(ctx, "zone_kind", zoneInfo.Kind)
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

	ctx = tflog.SetField(ctx, "zone_id", d.Id())
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
	if err := d.Set("soa_edit", zoneInfo.SoaEdit); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS SOA Edit: %w", err))
	}
	if err := d.Set("dnssec", zoneInfo.DNSSec); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS DNSSEC: %w", err))
	}
	if zoneInfo.APIRectify != nil {
		if err := d.Set("api_rectify", *zoneInfo.APIRectify); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS API Rectify: %w", err))
		}
	}
	if err := d.Set("serial", zoneInfo.Serial); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Serial: %w", err))
	}
	if err := d.Set("notified_serial", zoneInfo.NotifiedSerial); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Notified Serial: %w", err))
	}
	if err := d.Set("edited_serial", zoneInfo.EditedSerial); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Edited Serial: %w", err))
	}
	if err := d.Set("last_check", zoneInfo.LastCheck); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Last Check: %w", err))
	}
	for name, values := range map[string][]string{
		"master_tsig_key_ids": zoneInfo.MasterTsigKeyIDs,
		"slave_tsig_key_ids":  zoneInfo.SlaveTsigKeyIDs,
	} {
		if err := d.Set(name, values); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS %s: %w", name, err))
		}
	}

	if strings.EqualFold(zoneInfo.Kind, "Slave") {
		if err := d.Set("masters", zoneInfo.Masters); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS Masters: %w", err))
		}
	}

	return nil
}

func resourcePDNSZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Updating PowerDNS Zone")

	client := meta.(*ProviderClients)

	if d.HasChanges("kind", "account", "catalog", "soa_edit_api", "soa_edit", "dnssec", "api_rectify", "masters", "master_tsig_key_ids", "slave_tsig_key_ids") {
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
		// Only send these optional settings when changed, including false and
		// empty values, so unrelated updates preserve server-managed settings.
		if d.HasChange("soa_edit") {
			value := d.Get("soa_edit").(string)
			zoneInfo.SoaEdit = &value
		}
		if d.HasChange("dnssec") {
			value := d.Get("dnssec").(bool)
			zoneInfo.DNSSec = &value
		}
		if d.HasChange("api_rectify") {
			value := d.Get("api_rectify").(bool)
			zoneInfo.APIRectify = &value
		}
		// Pointer fields distinguish clearing an explicitly managed set from
		// omitting it during an unrelated zone update.
		if d.HasChange("master_tsig_key_ids") {
			values := zoneTSIGReferences(d, "master_tsig_key_ids")
			zoneInfo.MasterTsigKeyIDs = &values
		}
		if d.HasChange("slave_tsig_key_ids") {
			values := zoneTSIGReferences(d, "slave_tsig_key_ids")
			zoneInfo.SlaveTsigKeyIDs = &values
		}

		if err := client.PDNS.UpdateZone(ctx, d.Id(), zoneInfo); err != nil {
			return diag.FromErr(fmt.Errorf("error updating PowerDNS Zone: %w", err))
		}
	}

	return resourcePDNSZoneRead(ctx, d, meta)
}

func resourcePDNSZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ctx = tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Deleting PowerDNS Zone")

	if err := client.PDNS.DeleteZone(ctx, d.Id()); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS Zone: %w", err))
	}
	tflog.Info(ctx, "Deleted PowerDNS Zone")
	return nil
}
