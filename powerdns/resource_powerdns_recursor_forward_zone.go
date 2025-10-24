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

func resourcePDNSRecursorForwardZoneCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	zone := d.Get("zone").(string)
	servers := d.Get("servers").([]interface{})

	tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Creating recursor forward zone")

	// Get current forward-zones
	currentValue, err := client.GetRecursorConfigValue(ctx, "forward-zones")
	if err != nil {
		// Only treat "not found" as empty config, other errors should fail
		if errors.Is(err, ErrNotFound) {
			currentValue = ""
		} else {
			return diag.FromErr(fmt.Errorf("failed to get current forward-zones config: %w", err))
		}
	}

	// Parse current forward-zones
	forwardZones := parseForwardZones(currentValue)

	// Add/update zone
	serverList := make([]string, len(servers))
	for i, s := range servers {
		serverList[i] = s.(string)
	}
	forwardZones[zone] = serverList

	// Serialize back
	newValue := serializeForwardZones(forwardZones)

	if err := client.SetRecursorConfigValue(ctx, "forward-zones", newValue); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create recursor forward zone: %w", err))
	}

	d.SetId(zone)
	tflog.Info(ctx, "Created recursor forward zone", map[string]any{"id": zone})
	return resourcePDNSRecursorForwardZoneRead(ctx, d, meta)
}

func resourcePDNSRecursorForwardZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	zone := d.Id()

	tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Reading recursor forward zone")

	value, err := client.GetRecursorConfigValue(ctx, "forward-zones")
	if err != nil {
		// Only treat "not found" as removing from state, other errors should fail
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor forward-zones config not found; removing from state")
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to get forward-zones config: %w", err))
	}

	forwardZones := parseForwardZones(value)

	servers, exists := forwardZones[zone]
	if !exists {
		tflog.Warn(ctx, "Forward zone not found; removing from state")
		d.SetId("")
		return nil
	}

	if err := d.Set("zone", zone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone: %w", err))
	}
	if err := d.Set("servers", servers); err != nil {
		return diag.FromErr(fmt.Errorf("error setting servers: %w", err))
	}

	return nil
}

func resourcePDNSRecursorForwardZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	zone := d.Id()
	servers := d.Get("servers").([]interface{})

	tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Updating recursor forward zone")

	// Get current forward-zones
	currentValue, err := client.GetRecursorConfigValue(ctx, "forward-zones")
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to get current forward-zones: %w", err))
	}

	// Parse current forward-zones
	forwardZones := parseForwardZones(currentValue)

	// Update zone
	serverList := make([]string, len(servers))
	for i, s := range servers {
		serverList[i] = s.(string)
	}
	forwardZones[zone] = serverList

	// Serialize back
	newValue := serializeForwardZones(forwardZones)

	if err := client.SetRecursorConfigValue(ctx, "forward-zones", newValue); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update recursor forward zone: %w", err))
	}

	return resourcePDNSRecursorForwardZoneRead(ctx, d, meta)
}

func resourcePDNSRecursorForwardZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*Client)

	zone := d.Id()

	tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Deleting recursor forward zone")

	// Get current forward-zones
	currentValue, err := client.GetRecursorConfigValue(ctx, "forward-zones")
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to get current forward-zones: %w", err))
	}

	// Parse current forward-zones
	forwardZones := parseForwardZones(currentValue)

	// Remove zone
	delete(forwardZones, zone)

	// Serialize back
	newValue := serializeForwardZones(forwardZones)

	if err := client.SetRecursorConfigValue(ctx, "forward-zones", newValue); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting recursor forward zone: %w", err))
	}

	tflog.Info(ctx, "Successfully deleted recursor forward zone")
	return nil
}

// parseForwardZones parses the forward-zones string into a map
func parseForwardZones(value string) map[string][]string {
	result := make(map[string][]string)
	if value == "" {
		return result
	}

	entries := strings.Split(value, ";")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			zone := strings.TrimSpace(parts[0])
			serversStr := strings.TrimSpace(parts[1])
			servers := strings.Split(serversStr, ",")
			for i, s := range servers {
				servers[i] = strings.TrimSpace(s)
			}
			result[zone] = servers
		}
	}
	return result
}

// serializeForwardZones serializes the map back to forward-zones string
func serializeForwardZones(zones map[string][]string) string {
	var entries []string
	for zone, servers := range zones {
		entries = append(entries, zone+"="+strings.Join(servers, ","))
	}
	return strings.Join(entries, ";")
}
