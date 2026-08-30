package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// NormalizeTSIGKeyID returns the ID form of a TSIG key reference. PowerDNS
// identifies a key by its name followed by a trailing dot and rewrites a bare
// name to that form, so both spellings have to compare equal.
func NormalizeTSIGKeyID(keyID string) string {
	if keyID == "" || strings.HasSuffix(keyID, ".") {
		return keyID
	}
	return keyID + "."
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
			},
		},
	}
}

func resourcePDNSTSIGKeyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	tsigKey := TSIGKey{
		Name:      d.Get("name").(string),
		Algorithm: d.Get("algorithm").(string),
		Key:       d.Get("key").(string),
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

	tsigKey, err := client.PDNS.GetTSIGKey(ctx, d.Id())
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
	// Listing omits the secret, but a direct GET returns it. Keep whatever is
	// already in state when the API hands back an empty value.
	if tsigKey.Key != "" {
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

	if d.HasChanges("name", "key") {
		tsigKey := TSIGKey{
			Name: d.Get("name").(string),
			Key:  d.Get("key").(string),
		}

		updated, err := client.PDNS.UpdateTSIGKey(ctx, d.Id(), tsigKey)
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

	if err := client.PDNS.DeleteTSIGKey(ctx, d.Id()); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS TSIG key: %w", err))
	}

	tflog.Info(ctx, "Deleted PowerDNS TSIG key")
	return nil
}
