package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// writableRecursorSettings lists the settings the recursor API accepts a write
// for. Verified against Recursor 5.4.4: ws-recursor.cc registers handlers for
// /config/allow-from and /config/allow-notify-from only, and any other name
// returns 404 for both GET and PUT.
var writableRecursorSettings = []string{
	"allow-from",
	"allow-notify-from",
}

func resourcePDNSRecursorConfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSRecursorConfigCreate,
		ReadContext:   resourcePDNSRecursorConfigRead,
		UpdateContext: resourcePDNSRecursorConfigUpdate,
		DeleteContext: resourcePDNSRecursorConfigDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				// The recursor exposes exactly two settings for writing. Every
				// other name answers 404 on both GET and PUT, so accepting one
				// only defers the failure to apply.
				ValidateFunc: validation.StringInSlice(writableRecursorSettings, false),
				Description: "The name of the recursor config setting. The recursor API only " +
					"exposes allow-from and allow-notify-from for writing.",
			},
			"value": {
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required:    true,
				Description: "The value of the recursor config setting (list of CIDRs, etc.)",
			},
		},
	}
}

func resourcePDNSRecursorConfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	name := d.Get("name").(string)
	rawValues := d.Get("value").(*schema.Set).List()

	values := make([]string, len(rawValues))
	for i, v := range rawValues {
		values[i] = v.(string)
	}

	ctx = tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Creating recursor config")

	if err := recursorClient.SetConfig(ctx, name, values); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create recursor config %q: %w", name, err))
	}

	d.SetId(name)
	tflog.Info(ctx, "Created recursor config", map[string]any{"id": name})
	return resourcePDNSRecursorConfigRead(ctx, d, meta)
}

func resourcePDNSRecursorConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	name := d.Id()
	ctx = tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Reading recursor config")

	setting, err := recursorClient.GetConfig(ctx, name)
	if err != nil {
		// Only treat "not found" as removing from state, other errors should fail
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor config not found; removing from state")
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to get recursor config %q: %w", name, err))
	}

	if err := d.Set("name", setting.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting name: %w", err))
	}
	if err := d.Set("value", setting.Value); err != nil {
		return diag.FromErr(fmt.Errorf("error setting value: %w", err))
	}

	return nil
}

func resourcePDNSRecursorConfigUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	name := d.Id()
	rawValues := d.Get("value").(*schema.Set).List()

	values := make([]string, len(rawValues))
	for i, v := range rawValues {
		values[i] = v.(string)
	}

	ctx = tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Updating recursor config")

	if err := recursorClient.SetConfig(ctx, name, values); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update recursor config %q: %w", name, err))
	}

	return resourcePDNSRecursorConfigRead(ctx, d, meta)
}

func resourcePDNSRecursorConfigDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Id()
	ctx = tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Deleting recursor config")

	// The API only supports GET and PUT for config, so delete will do nothing
	tflog.Info(ctx, "Successfully deleted recursor config (removed from state)", map[string]any{"name": name})
	return nil
}
