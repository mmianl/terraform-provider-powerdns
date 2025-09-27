package powerdns

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSRecursorForwardZone() *schema.Resource {
	return &schema.Resource{
		Create: resourcePDNSRecursorForwardZoneCreate,
		Read:   resourcePDNSRecursorForwardZoneRead,
		Update: resourcePDNSRecursorForwardZoneUpdate,
		Delete: resourcePDNSRecursorForwardZoneDelete,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 255),
				Description: "The zone name to forward. Note: In PowerDNS Recursor 5.3+, " +
					"forward-zones configuration is read-only via the API. Only 'incoming.allow_from' and 'incoming.allow_notify_from' can be modified.",
			},

			"servers": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of DNS servers to forward queries to. " +
					"For read-only configurations, this will be ignored and the existing value will be used.",
			},
		},
	}
}

func resourcePDNSRecursorForwardZoneCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	zone := d.Get("zone").(string)
	servers := d.Get("servers").([]interface{})

	log.Printf("[INFO] Creating recursor forward zone: %s", zone)
	log.Printf("[DEBUG] Servers: %+v", servers)

	// Get current forward-zones
	currentValue, err := client.GetRecursorConfigValue("forward-zones")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			currentValue = ""
			log.Printf("[DEBUG] No existing forward-zones config found, starting with empty")
		} else {
			log.Printf("[DEBUG] Error getting forward-zones: %s", err)
			return fmt.Errorf("failed to get current forward-zones config: %s", err)
		}
	}

	log.Printf("[DEBUG] Current forward-zones value: '%s'", currentValue)

	// Parse current forward-zones
	forwardZones := parseForwardZones(currentValue)
	log.Printf("[DEBUG] Parsed forward zones: %+v", forwardZones)

	// Add new zone
	serverList := make([]string, len(servers))
	for i, s := range servers {
		serverList[i] = s.(string)
	}
	forwardZones[zone] = serverList
	log.Printf("[DEBUG] Updated forward zones map: %+v", forwardZones)

	// Serialize back
	newValue := serializeForwardZones(forwardZones)
	log.Printf("[DEBUG] New forward-zones value to set: '%s'", newValue)

	err = client.SetRecursorConfigValue("forward-zones", newValue)
	if err != nil {
		log.Printf("[DEBUG] SetRecursorConfigValue error: %s", err)
		// Check if this is a 404 error indicating the forward-zones config is not supported
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "404") {
			return fmt.Errorf("failed to create recursor forward zone: forward-zones configuration is not supported by the PowerDNS recursor server (HTTP 404). Note: Some settings are read-only or startup-only")
		}
		return fmt.Errorf("failed to create recursor forward zone: %s", err)
	}

	d.SetId(zone)
	log.Printf("[INFO] Created recursor forward zone with ID: %s", d.Id())
	return resourcePDNSRecursorForwardZoneRead(d, meta)
}

func resourcePDNSRecursorForwardZoneRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	zone := d.Id()

	log.Printf("[INFO] Reading recursor forward zone: %s", zone)

	value, err := client.GetRecursorConfigValue("forward-zones")
	if err != nil {
		// Check if this is a 404 error indicating the config is not supported
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "404") {
			log.Printf("[WARN] Recursor forward-zones config is not supported (404), using configured values")
			// For unsupported configs, use the configured values
			zone := d.Get("zone").(string)
			servers := d.Get("servers").([]interface{})

			err = d.Set("zone", zone)
			if err != nil {
				return fmt.Errorf("error setting zone: %s", err)
			}

			err = d.Set("servers", servers)
			if err != nil {
				return fmt.Errorf("error setting servers: %s", err)
			}

			return nil
		} else if errors.Is(err, ErrNotFound) {
			log.Printf("[WARN] Recursor forward-zones config not found, removing from state: %s", zone)
			d.SetId("")
			return nil
		} else {
			// Real error (auth, connection, server error, etc.)
			return fmt.Errorf("failed to get forward-zones config: %s", err)
		}
	}

	forwardZones := parseForwardZones(value)

	servers, exists := forwardZones[zone]
	if !exists {
		log.Printf("[WARN] Forward zone not found, removing from state: %s", zone)
		d.SetId("")
		return nil
	}

	err = d.Set("zone", zone)
	if err != nil {
		return fmt.Errorf("error setting zone: %s", err)
	}

	err = d.Set("servers", servers)
	if err != nil {
		return fmt.Errorf("error setting servers: %s", err)
	}

	return nil
}

func resourcePDNSRecursorForwardZoneUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	zone := d.Id()
	servers := d.Get("servers").([]interface{})

	log.Printf("[INFO] Updating recursor forward zone: %s", zone)

	// Get current forward-zones
	currentValue, err := client.GetRecursorConfigValue("forward-zones")
	if err != nil {
		return fmt.Errorf("failed to get current forward-zones: %s", err)
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

	err = client.SetRecursorConfigValue("forward-zones", newValue)
	if err != nil {
		// Check if this is a 404 error indicating the forward-zones config is not supported
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "404") {
			return fmt.Errorf("failed to update recursor forward zone: forward-zones configuration is not supported by the PowerDNS recursor server (HTTP 404). Note: Some settings are read-only or startup-only")
		}
		return fmt.Errorf("failed to update recursor forward zone: %s", err)
	}

	return resourcePDNSRecursorForwardZoneRead(d, meta)
}

func resourcePDNSRecursorForwardZoneDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	zone := d.Id()

	log.Printf("[INFO] Deleting recursor forward zone: %s", zone)

	// Get current forward-zones
	currentValue, err := client.GetRecursorConfigValue("forward-zones")
	if err != nil {
		return fmt.Errorf("failed to get current forward-zones: %s", err)
	}

	// Parse current forward-zones
	forwardZones := parseForwardZones(currentValue)

	// Remove zone
	delete(forwardZones, zone)

	// Serialize back
	newValue := serializeForwardZones(forwardZones)

	err = client.SetRecursorConfigValue("forward-zones", newValue)
	if err != nil {
		// Check if this is a 404 error indicating the forward-zones config is not supported
		if strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "404") {
			return fmt.Errorf("error deleting recursor forward zone: forward-zones configuration is not supported by the PowerDNS recursor server (HTTP 404). Note: Some settings are read-only or startup-only")
		}
		return fmt.Errorf("error deleting recursor forward zone: %s", err)
	}

	log.Printf("[INFO] Successfully deleted recursor forward zone")
	return nil
}

// parseForwardZones parses the forward-zones string into a map
func parseForwardZones(value string) map[string][]string {
	result := make(map[string][]string)
	if value == "" {
		return result
	}

	// PowerDNS recursor forward-zones format is: "zone1=server1,server2;zone2=server3,server4"
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
			if serversStr == "" {
				continue
			}
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
