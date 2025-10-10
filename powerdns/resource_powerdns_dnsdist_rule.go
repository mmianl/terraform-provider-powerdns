package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourcePDNSDNSdistRule returns a resource for DNSdist rules
func resourcePDNSDNSdistRule() *schema.Resource {
	return &schema.Resource{
		Create: resourcePDNSDNSdistRuleCreate,
		Read:   resourcePDNSDNSdistRuleRead,
		Update: resourcePDNSDNSdistRuleUpdate,
		Delete: resourcePDNSDNSdistRuleDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the DNSdist rule",
			},
			"rule": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "DNSdist rule expression",
			},
			"action": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Action to take when rule matches",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the rule is enabled",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the rule",
			},
		},
	}
}

func resourcePDNSDNSdistRuleCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	rule := DNSdistRule{
		Name:        d.Get("name").(string),
		Rule:        d.Get("rule").(string),
		Action:      d.Get("action").(string),
		Enabled:     d.Get("enabled").(bool),
		Description: d.Get("description").(string),
	}

	ctx := context.Background()
	tflog.Debug(ctx, "Creating DNSdist rule", map[string]interface{}{
		"ruleName": rule.Name,
	})

	createdRule, err := client.CreateDNSdistRule(ctx, rule)
	if err != nil {
		return fmt.Errorf("error creating DNSdist rule: %s", err)
	}

	d.SetId(createdRule.ID)

	return resourcePDNSDNSdistRuleRead(d, meta)
}

func resourcePDNSDNSdistRuleRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	ruleID := d.Id()

	ctx := context.Background()
	tflog.Debug(ctx, "Reading DNSdist rule", map[string]interface{}{
		"ruleID": ruleID,
	})

	rules, err := client.GetDNSdistRules(ctx)
	if err != nil {
		return fmt.Errorf("error reading DNSdist rules: %s", err)
	}

	var foundRule *DNSdistRule
	for _, rule := range rules {
		if rule.ID == ruleID {
			foundRule = &rule
			break
		}
	}

	if foundRule == nil {
		d.SetId("")
		return nil
	}

	if err := d.Set("name", foundRule.Name); err != nil {
		return fmt.Errorf("error setting name: %s", err)
	}
	if err := d.Set("rule", foundRule.Rule); err != nil {
		return fmt.Errorf("error setting rule: %s", err)
	}
	if err := d.Set("action", foundRule.Action); err != nil {
		return fmt.Errorf("error setting action: %s", err)
	}
	if err := d.Set("enabled", foundRule.Enabled); err != nil {
		return fmt.Errorf("error setting enabled: %s", err)
	}
	if err := d.Set("description", foundRule.Description); err != nil {
		return fmt.Errorf("error setting description: %s", err)
	}

	return nil
}

func resourcePDNSDNSdistRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	// For now, we'll implement this as a delete and create since DNSdist
	// rules are typically identified by their position/order rather than ID
	// In a production implementation, you might want to handle this differently

	client := meta.(*Client)
	ruleID := d.Id()

	ctx := context.Background()
	tflog.Debug(ctx, "Updating DNSdist rule", map[string]interface{}{
		"ruleID": ruleID,
	})

	// Delete the old rule
	err := client.DeleteDNSdistRule(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("error deleting old DNSdist rule: %s", err)
	}

	// Create the new rule
	rule := DNSdistRule{
		Name:        d.Get("name").(string),
		Rule:        d.Get("rule").(string),
		Action:      d.Get("action").(string),
		Enabled:     d.Get("enabled").(bool),
		Description: d.Get("description").(string),
	}

	createdRule, err := client.CreateDNSdistRule(ctx, rule)
	if err != nil {
		return fmt.Errorf("error creating updated DNSdist rule: %s", err)
	}

	d.SetId(createdRule.ID)

	return resourcePDNSDNSdistRuleRead(d, meta)
}

func resourcePDNSDNSdistRuleDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	ruleID := d.Id()

	ctx := context.Background()
	tflog.Debug(ctx, "Deleting DNSdist rule", map[string]interface{}{
		"ruleID": ruleID,
	})

	err := client.DeleteDNSdistRule(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("error deleting DNSdist rule: %s", err)
	}

	d.SetId("")
	return nil
}
