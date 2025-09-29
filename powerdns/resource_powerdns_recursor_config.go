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

func resourcePDNSRecursorConfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSRecursorConfigCreate,
		ReadContext:   resourcePDNSRecursorConfigRead,
		UpdateContext: resourcePDNSRecursorConfigUpdate,
		DeleteContext: resourcePDNSRecursorConfigDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 255),
				Description:  "The name of the recursor config setting",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The value of the recursor config setting",
			},
		},
	}
}

func resourcePDNSRecursorConfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	name := d.Get("name").(string)
	value := d.Get("value").(string)

	tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Creating recursor config")

	if err := client.SetRecursorConfigValue(ctx, name, value); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create recursor config: %w", err))
	}

	d.SetId(name)
	tflog.Info(ctx, "Created recursor config", map[string]any{"id": name})
	return resourcePDNSRecursorConfigRead(ctx, d, meta)
}

func resourcePDNSRecursorConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	name := d.Id()
	tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Reading recursor config")

	value, err := client.GetRecursorConfigValue(ctx, name)
	if err != nil {
		// Only treat "not found" as removing from state, other errors should fail
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor config not found; removing from state")
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to get recursor config: %w", err))
	}

	if err := d.Set("name", name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting name: %w", err))
	}
	if err := d.Set("value", value); err != nil {
		return diag.FromErr(fmt.Errorf("error setting value: %w", err))
	}

	return nil
}

func resourcePDNSRecursorConfigUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	name := d.Id()
	value := d.Get("value").(string)

	tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Updating recursor config")

	if err := client.SetRecursorConfigValue(ctx, name, value); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update recursor config: %w", err))
	}

	return resourcePDNSRecursorConfigRead(ctx, d, meta)
}

func resourcePDNSRecursorConfigDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	name := d.Id()
	tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Deleting recursor config")

	if err := client.DeleteRecursorConfigValue(ctx, name); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting recursor config: %w", err))
	}

	tflog.Info(ctx, "Successfully deleted recursor config")
	return nil
}
