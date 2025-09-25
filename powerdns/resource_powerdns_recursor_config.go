package powerdns

import (
	"errors"
	"fmt"
	"log"

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

func resourcePDNSRecursorConfigCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	name := d.Get("name").(string)
	value := d.Get("value").(string)

	log.Printf("[INFO] Creating recursor config: %s", name)

	err := client.SetRecursorConfigValue(name, value)
	if err != nil {
		return fmt.Errorf("failed to create recursor config: %s", err)
	}

	d.SetId(name)
	log.Printf("[INFO] Created recursor config with ID: %s", d.Id())
	return resourcePDNSRecursorConfigRead(d, meta)
}

func resourcePDNSRecursorConfigRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	name := d.Id()

	log.Printf("[INFO] Reading recursor config: %s", name)

	value, err := client.GetRecursorConfigValue(name)
	if err != nil {
		// Only treat "not found" as removing from state, other errors should fail
		if errors.Is(err, ErrNotFound) {
			log.Printf("[WARN] Recursor config not found, removing from state: %s", name)
			d.SetId("")
			return nil
		} else {
			// Real error (auth, connection, server error, etc.)
			return fmt.Errorf("failed to get recursor config: %s", err)
		}
	}

	err = d.Set("name", name)
	if err != nil {
		return fmt.Errorf("error setting name: %s", err)
	}

	err = d.Set("value", value)
	if err != nil {
		return fmt.Errorf("error setting value: %s", err)
	}

	return nil
}

func resourcePDNSRecursorConfigUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	name := d.Id()
	value := d.Get("value").(string)

	log.Printf("[INFO] Updating recursor config: %s", name)

	err := client.SetRecursorConfigValue(name, value)
	if err != nil {
		return fmt.Errorf("failed to update recursor config: %s", err)
	}

	return resourcePDNSRecursorConfigRead(d, meta)
}

func resourcePDNSRecursorConfigDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	name := d.Id()

	log.Printf("[INFO] Deleting recursor config: %s", name)

	err := client.DeleteRecursorConfigValue(name)
	if err != nil {
		return fmt.Errorf("error deleting recursor config: %s", err)
	}

	log.Printf("[INFO] Successfully deleted recursor config")
	return nil
}
