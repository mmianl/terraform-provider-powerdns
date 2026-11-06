package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// tsigAlgorithms lists the algorithms accepted by the PowerDNS API.
var tsigAlgorithms = []string{
	"hmac-md5",
	"hmac-sha1",
	"hmac-sha224",
	"hmac-sha256",
	"hmac-sha384",
	"hmac-sha512",
}

func resourcePDNSTSIGKey() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSTSIGKeyCreate,
		ReadContext:   resourcePDNSTSIGKeyRead,
		UpdateContext: resourcePDNSTSIGKeyUpdate,
		DeleteContext: resourcePDNSTSIGKeyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the TSIG key.",
				// PowerDNS returns names without their optional trailing dot.
				// Normalize before planning so dependent resources see the same
				// name before and after key creation.
				StateFunc: func(value interface{}) string {
					return strings.TrimSuffix(value.(string), ".")
				},
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.TrimSuffix(old, ".") == strings.TrimSuffix(new, ".")
				},
			},

			"algorithm": {
				Type:     schema.TypeString,
				Required: true,
				// PowerDNS stores a changed algorithm as an extra key under the
				// same name instead of replacing the existing one, so the key has
				// to be recreated rather than updated in place.
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice(tsigAlgorithms, false),
				Description:  "The TSIG algorithm, e.g. hmac-sha256. Changing this forces a new key.",
			},

			"key": {
				Type:      schema.TypeString,
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				Description: "The base64 encoded secret. When omitted, PowerDNS generates " +
					"the key material and it is stored in state.",
				ConflictsWith: []string{"key_wo"},
			},

			"key_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ConflictsWith: []string{"key"},
				RequiredWith:  []string{"key_wo_version"},
				Description: "The base64 encoded secret, as a write-only value that is never saved " +
					"to the plan or state. Needs Terraform 1.11 or later. Change key_wo_version to send a new value.",
			},

			"key_wo_version": {
				Type:         schema.TypeInt,
				Optional:     true,
				RequiredWith: []string{"key_wo"},
				ValidateFunc: validation.IntAtLeast(1),
				Description: "Bump this to make the provider send the current key_wo value to PowerDNS again. " +
					"Terraform cannot see a write-only value change, so this is how a rotation is requested.",
			},
		},
	}
}

func resourcePDNSTSIGKeyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	key, diags := tsigKeyFromConfig(d)
	if diags.HasError() {
		return diags
	}

	tsigKey := TSIGKey{
		Name:      d.Get("name").(string),
		Algorithm: d.Get("algorithm").(string),
		Key:       key,
	}

	ctx = tflog.SetField(ctx, "tsigkey_name", tsigKey.Name)
	tflog.Debug(ctx, "Creating PowerDNS TSIG key")

	created, err := client.PDNS.CreateTSIGKey(ctx, tsigKey)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(created.ID)
	tflog.Info(ctx, "Created PowerDNS TSIG key", map[string]any{"id": created.ID})
	return resourcePDNSTSIGKeyRead(ctx, d, meta)
}

func resourcePDNSTSIGKeyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ctx = tflog.SetField(ctx, "tsigkey_id", d.Id())
	tflog.Debug(ctx, "Reading PowerDNS TSIG key")

	tsigKey, err := client.PDNS.GetTSIGKey(ctx, d.Id(), d.Get("algorithm").(string))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "TSIG key not found; removing from state")
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS TSIG key: %w", err))
	}

	if err := d.Set("name", tsigKey.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS TSIG key name: %w", err))
	}
	if err := d.Set("algorithm", tsigKey.Algorithm); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS TSIG key algorithm: %w", err))
	}
	// Switching to write-only mode must also remove a previously persisted
	// generated or explicit secret, even if the API omits it on this read.
	if d.Get("key_wo_version").(int) != 0 {
		if err := d.Set("key", nil); err != nil {
			return diag.FromErr(fmt.Errorf("error clearing PowerDNS TSIG key material: %w", err))
		}
	} else if tsigKey.Key != "" {
		// Listing omits the secret, but a direct GET returns it. Keep whatever
		// is already in state when the API hands back an empty value.
		if err := d.Set("key", tsigKey.Key); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS TSIG key material: %w", err))
		}
	}

	return nil
}

func resourcePDNSTSIGKeyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ctx = tflog.SetField(ctx, "tsigkey_id", d.Id())
	tflog.Debug(ctx, "Updating PowerDNS TSIG key")

	if d.HasChanges("name", "key", "key_wo_version") {
		tsigKey := TSIGKey{
			Name: d.Get("name").(string),
		}
		// A rename can omit the secret. In write-only mode, only a version
		// change requests rotation, even if the configured secret also changed.
		rotate := d.HasChange("key")
		if d.Get("key_wo_version").(int) != 0 {
			rotate = d.HasChange("key_wo_version")
		}
		if rotate {
			key, diags := tsigKeyFromConfig(d)
			if diags.HasError() {
				return diags
			}
			tsigKey.Key = key
		}

		updated, err := client.PDNS.UpdateTSIGKey(ctx, d.Id(), d.Get("algorithm").(string), tsigKey)
		if err != nil {
			return diag.FromErr(fmt.Errorf("error updating PowerDNS TSIG key: %w", err))
		}

		// Renaming a key changes its ID, so follow it to keep later reads working.
		if updated.ID != "" {
			d.SetId(updated.ID)
		}
	}

	return resourcePDNSTSIGKeyRead(ctx, d, meta)
}

func resourcePDNSTSIGKeyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ctx = tflog.SetField(ctx, "tsigkey_id", d.Id())
	tflog.Debug(ctx, "Deleting PowerDNS TSIG key")

	if err := client.PDNS.DeleteTSIGKey(ctx, d.Id(), d.Get("algorithm").(string)); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS TSIG key: %w", err))
	}

	tflog.Info(ctx, "Deleted PowerDNS TSIG key")
	return nil
}

// tsigKeyFromConfig returns the secret to send to PowerDNS. A write-only value
// exists only in the configuration, never in state, so it is read from there.
func tsigKeyFromConfig(d *schema.ResourceData) (string, diag.Diagnostics) {
	if d.Get("key_wo_version").(int) != 0 {
		wo, diags := d.GetRawConfigAt(cty.GetAttrPath("key_wo"))
		if diags.HasError() {
			return "", diags
		}
		if !wo.IsNull() && wo.IsKnown() {
			return wo.AsString(), nil
		}
	}
	return d.Get("key").(string), nil
}
