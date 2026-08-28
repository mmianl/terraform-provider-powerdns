package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePDNSViewZoneAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSViewZoneAssociationCreate,
		ReadContext:   resourcePDNSViewZoneAssociationRead,
		DeleteContext: resourcePDNSViewZoneAssociationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSViewZoneAssociationImport,
		},
		Schema: map[string]*schema.Schema{
			"view": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateViewName,
			},
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateZoneName,
			},
		},
	}
}

func viewZoneAssociationID(view string, zone string) string {
	return view + idSeparator + zone
}

func parseViewZoneAssociationID(id string) (string, string, error) {
	parts := strings.Split(id, idSeparator)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid view zone association id %q, expected <view>%s<zone>", id, idSeparator)
	}
	return parts[0], parts[1], nil
}

func resourcePDNSViewZoneAssociationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	view := d.Get("view").(string)
	zone := d.Get("zone").(string)

	ctx = tflog.SetField(ctx, "view", view)
	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Creating PowerDNS view zone association")

	if err := client.PDNS.AddZoneToView(ctx, view, zone); err != nil {
		return diag.FromErr(fmt.Errorf("failed to add zone %s to view %s: %w", zone, view, err))
	}

	d.SetId(viewZoneAssociationID(view, zone))
	return resourcePDNSViewZoneAssociationRead(ctx, d, meta)
}

func resourcePDNSViewZoneAssociationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	view, zone, err := parseViewZoneAssociationID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "view", view)
	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Reading PowerDNS view zone association")

	pdnsView, err := client.PDNS.GetView(ctx, view)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to get view %s: %w", view, err))
	}

	for _, existingZone := range pdnsView.Zones {
		if existingZone == zone {
			if err := d.Set("view", view); err != nil {
				return diag.FromErr(fmt.Errorf("error setting view: %w", err))
			}
			if err := d.Set("zone", zone); err != nil {
				return diag.FromErr(fmt.Errorf("error setting zone: %w", err))
			}
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourcePDNSViewZoneAssociationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	view, zone, err := parseViewZoneAssociationID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "view", view)
	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Deleting PowerDNS view zone association")

	if err := client.PDNS.RemoveZoneFromView(ctx, view, zone); err != nil && !errors.Is(err, ErrNotFound) {
		return diag.FromErr(fmt.Errorf("failed to remove zone %s from view %s: %w", zone, view, err))
	}

	d.SetId("")
	return nil
}

func resourcePDNSViewZoneAssociationImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	view, zone, err := parseViewZoneAssociationID(d.Id())
	if err != nil {
		return nil, err
	}
	if err := d.Set("view", view); err != nil {
		return nil, err
	}
	if err := d.Set("zone", zone); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
