package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePDNSRecursorForwardZone() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSRecursorForwardZoneCreate,
		ReadContext:   resourcePDNSRecursorForwardZoneRead,
		UpdateContext: resourcePDNSRecursorForwardZoneUpdate,
		DeleteContext: resourcePDNSRecursorForwardZoneDelete,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					value := v.(string)
					if len(value) < 1 || len(value) > 255 {
						errors = append(errors, fmt.Errorf("%q must be between 1 and 255 characters", k))
					}
					if !strings.HasSuffix(value, ".") {
						errors = append(errors, fmt.Errorf("%q must be a fully qualified domain name ending with a dot", k))
					}
					return
				},
				Description: "The zone name to forward. Must be a fully qualified domain name (FQDN) ending with a trailing dot (e.g., \"example.com.\").",
			},
			"servers": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of DNS servers to forward queries to",
			},
		},
	}
}

func getRecursorClient(meta interface{}) (*RecursorClient, diag.Diagnostics) {
	clients := meta.(*ProviderClients)
	if clients.Recursor == nil {
		return nil, diag.Diagnostics{{
			Severity: diag.Error,
			Summary:  "Recursor client not configured",
			Detail:   "The 'recursor_server_url' provider argument must be set to manage recursor forward zones.",
		}}
	}
	return clients.Recursor, nil
}

func resourcePDNSRecursorForwardZoneCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	zoneName := d.Get("zone").(string)
	rawServers := d.Get("servers").([]interface{})

	tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Creating recursor forward zone")

	servers := make([]string, len(rawServers))
	for i, s := range rawServers {
		servers[i] = s.(string)
	}

	zone := &RecursorForwardZone{
		Name:             zoneName,
		Type:             "Zone",
		Kind:             "Forwarded",
		Servers:          servers,
		RecursionDesired: false,
	}

	if err := recursorClient.CreateForwardZone(ctx, zone); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create recursor forward zone %q: %w", zoneName, err))
	}

	d.SetId(zoneName)
	tflog.Info(ctx, "Created recursor forward zone", map[string]any{"id": zoneName})
	return resourcePDNSRecursorForwardZoneRead(ctx, d, meta)
}

func resourcePDNSRecursorForwardZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	zoneName := d.Id()

	tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Reading recursor forward zone")

	zone, err := recursorClient.GetForwardZone(ctx, zoneName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor forward zone not found; removing from state", map[string]any{"zone": zoneName})
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read recursor forward zone %q: %w", zoneName, err))
	}

	if err := d.Set("zone", zone.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone: %w", err))
	}
	if err := d.Set("servers", zone.Servers); err != nil {
		return diag.FromErr(fmt.Errorf("error setting servers: %w", err))
	}

	return nil
}

func resourcePDNSRecursorForwardZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	zoneName := d.Id()
	rawServers := d.Get("servers").([]interface{})

	tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Updating recursor forward zone")

	servers := make([]string, len(rawServers))
	for i, s := range rawServers {
		servers[i] = s.(string)
	}

	// update as delete then create
	if err := recursorClient.DeleteForwardZone(ctx, zoneName); err != nil && !errors.Is(err, ErrNotFound) {
		return diag.FromErr(fmt.Errorf("failed to delete existing recursor forward zone %q before update: %w", zoneName, err))
	}

	zone := &RecursorForwardZone{
		Name:             zoneName,
		Type:             "Zone",
		Kind:             "Forwarded",
		Servers:          servers,
		RecursionDesired: false,
	}

	if err := recursorClient.CreateForwardZone(ctx, zone); err != nil {
		return diag.FromErr(fmt.Errorf("failed to recreate recursor forward zone %q during update: %w", zoneName, err))
	}

	return resourcePDNSRecursorForwardZoneRead(ctx, d, meta)
}

func resourcePDNSRecursorForwardZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	recursorClient, diags := getRecursorClient(meta)
	if diags != nil {
		return diags
	}

	zoneName := d.Id()

	tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Deleting recursor forward zone")

	err := recursorClient.DeleteForwardZone(ctx, zoneName)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return diag.FromErr(fmt.Errorf("error deleting recursor forward zone %q: %w", zoneName, err))
	}

	tflog.Info(ctx, "Successfully deleted recursor forward zone", map[string]any{"zone": zoneName})
	return nil
}
