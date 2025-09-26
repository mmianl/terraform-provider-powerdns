package powerdns

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSRecursorConfig() *schema.Resource {
	return &schema.Resource{
		Create: resourcePDNSRecursorConfigCreate,
		Read:   resourcePDNSRecursorConfigRead,
		Update: resourcePDNSRecursorConfigUpdate,
		Delete: resourcePDNSRecursorConfigDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 255),
				Description: "The name of the recursor config setting. Note: In PowerDNS Recursor 5.3+, " +
					"ONLY 'incoming.allow_from' and 'incoming.allow_notify_from' can be modified via the API. " +
					"All other settings are read-only. The API is primarily for dynamic access control, not full configuration management.",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The value of the recursor config setting. For read-only settings, this will be ignored and the existing value will be used.",
			},
		},
	}
}

func resourcePDNSRecursorConfigCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	name := d.Get("name").(string)
	value := d.Get("value").(string)

	log.Printf("[INFO] Creating recursor config: %s with value: %s", name, value)
	log.Printf("[DEBUG] Create: Initial state - ID: '%s'", d.Id())

	// First check if the config exists and is readable
	existingValue, err := client.GetRecursorConfigValue(name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		log.Printf("[DEBUG] Create: GetRecursorConfigValue returned error: %s", err)
		return fmt.Errorf("failed to check existing recursor config %s: %s", name, err)
	}

	// If it exists and has the same value, we can just set the ID and return
	if err == nil && existingValue == value {
		log.Printf("[INFO] Recursor config %s already has the desired value: %s", name, value)
		log.Printf("[DEBUG] Create: Setting ID to: %s", name)
		d.SetId(name)
		log.Printf("[DEBUG] Create: Calling Read after setting existing value")
		return resourcePDNSRecursorConfigRead(d, meta)
	}

	// Try to set the config value
	err = client.SetRecursorConfigValue(name, value)
	if err != nil {
		log.Printf("[DEBUG] Create: SetRecursorConfigValue returned error: %s", err)
		// Check if this is a read-only configuration error
		if isReadOnlyConfigError(err) {
			log.Printf("[WARN] Recursor config %s appears to be read-only, will use existing value", name)
			log.Printf("[INFO] This is normal for settings like query-local-address, local-address, or version")
			// For read-only configs, we'll still set the ID but use the existing value
			log.Printf("[DEBUG] Create: Setting ID to: %s (read-only config)", name)
			d.SetId(name)
			log.Printf("[DEBUG] Create: Calling Read for read-only config")
			return resourcePDNSRecursorConfigRead(d, meta)
		}
		return fmt.Errorf("failed to create recursor config %s: %s. Note: Some settings are read-only or startup-only", name, err)
	}

	log.Printf("[DEBUG] Create: SetRecursorConfigValue succeeded, setting ID to: %s", name)
	d.SetId(name)
	log.Printf("[INFO] Created recursor config with ID: %s", d.Id())
	log.Printf("[DEBUG] Create: Calling Read after successful set")
	return resourcePDNSRecursorConfigRead(d, meta)
}

func resourcePDNSRecursorConfigRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	name := d.Id()

	log.Printf("[INFO] Reading recursor config: %s", name)
	log.Printf("[DEBUG] Read: Current ID: '%s'", d.Id())
	log.Printf("[DEBUG] Read: Current name attribute: '%s'", d.Get("name"))
	log.Printf("[DEBUG] Read: Current value attribute: '%s'", d.Get("value"))

	value, err := client.GetRecursorConfigValue(name)
	if err != nil {
		log.Printf("[DEBUG] Read: GetRecursorConfigValue returned error: %s", err)
		// Check if this is a 404 error indicating the config is not supported
		if isReadOnlyConfigError(err) {
			log.Printf("[WARN] Recursor config %s is not supported (read-only/404), using configured value", name)
			log.Printf("[DEBUG] Read: Using configured value for read-only config")
			// For read-only/unsupported configs, use the configured value
			value = d.Get("value").(string)
			log.Printf("[DEBUG] Read: Successfully set value from configured value: '%s'", value)
		} else if errors.Is(err, ErrNotFound) {
			log.Printf("[WARN] Recursor config not found, removing from state: %s", name)
			log.Printf("[DEBUG] Read: Setting ID to empty (not found)")
			d.SetId("")
			return nil
		} else {
			log.Printf("[DEBUG] Read: Returning error for unexpected error")
			return fmt.Errorf("failed to get recursor config %s: %s", name, err)
		}
	} else {
		log.Printf("[DEBUG] Read: GetRecursorConfigValue succeeded, got value: '%s'", value)
	}

	log.Printf("[DEBUG] Read: Setting name to: %s", name)
	if err := d.Set("name", name); err != nil {
		log.Printf("[DEBUG] Read: Error setting name: %s", err)
		return fmt.Errorf("error setting name: %s", err)
	}

	log.Printf("[DEBUG] Read: Setting value to: %s", value)
	if err := d.Set("value", value); err != nil {
		log.Printf("[DEBUG] Read: Error setting value: %s", err)
		return fmt.Errorf("error setting value: %s", err)
	}

	log.Printf("[DEBUG] Read: Successfully completed")
	return nil
}

func resourcePDNSRecursorConfigUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	name := d.Id()
	value := d.Get("value").(string)

	log.Printf("[INFO] Updating recursor config: %s to value: %s", name, value)

	err := client.SetRecursorConfigValue(name, value)
	if err != nil {
		// Check if this is a read-only configuration error
		if isReadOnlyConfigError(err) {
			log.Printf("[WARN] Recursor config %s appears to be read-only, will keep existing value", name)
			log.Printf("[INFO] This is normal for settings like query-local-address, local-address, or version")
			// For read-only configs, we'll just refresh from the current value
			return resourcePDNSRecursorConfigRead(d, meta)
		}
		return fmt.Errorf("failed to update recursor config %s: %s. Note: Some settings are read-only or startup-only", name, err)
	}

	return resourcePDNSRecursorConfigRead(d, meta)
}

func resourcePDNSRecursorConfigDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	name := d.Id()

	log.Printf("[INFO] Deleting recursor config: %s", name)

	err := client.DeleteRecursorConfigValue(name)
	if err != nil {
		// If the config doesn't exist, consider it already deleted
		if errors.Is(err, ErrNotFound) {
			log.Printf("[WARN] Recursor config %s already deleted", name)
			return nil
		}
		return fmt.Errorf("error deleting recursor config %s: %s", name, err)
	}

	log.Printf("[INFO] Successfully deleted recursor config: %s", name)
	return nil
}

// isReadOnlyConfigError checks if an error indicates a read-only configuration
func isReadOnlyConfigError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	log.Printf("[DEBUG] isReadOnlyConfigError: Checking error message: %s", errMsg)

	// Check for HTTP 404 errors which indicate the config endpoint doesn't exist
	// This is common for read-only or startup-only configuration values
	if strings.Contains(errMsg, "HTTP 404") || strings.Contains(errMsg, "404") {
		log.Printf("[DEBUG] isReadOnlyConfigError: Detected 404 error, treating as read-only")
		return true
	}

	// Common read-only configuration error patterns
	readOnlyPatterns := []string{
		"read.?only",
		"read only",
		"cannot be set",
		"permission denied",
		"forbidden",
		"not allowed",
		"immutable",
		"static",
		"startup.?only",
		"runtime.?only",
	}

	for _, pattern := range readOnlyPatterns {
		if strings.Contains(strings.ToLower(errMsg), pattern) {
			log.Printf("[DEBUG] isReadOnlyConfigError: Matched pattern '%s', treating as read-only", pattern)
			return true
		}
	}

	log.Printf("[DEBUG] isReadOnlyConfigError: No read-only patterns matched, returning false")
	return false
}
