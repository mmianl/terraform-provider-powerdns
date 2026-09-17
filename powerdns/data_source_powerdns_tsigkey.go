package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSTSIGKey() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSTSIGKeyRead,

		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the TSIG key to retrieve, e.g. \"mykey.\".",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the TSIG key",
			},
			"algorithm": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The TSIG algorithm of the key",
			},
			"key": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The base64 encoded secret of the key",
			},
		},
	}
}

func dataSourcePDNSTSIGKeyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	keyID := d.Get("id").(string)
	ctx = tflog.SetField(ctx, "tsigkey_id", keyID)
	tflog.Info(ctx, "Reading TSIG key data source")

	tsigKey, err := client.PDNS.GetTSIGKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return diag.FromErr(fmt.Errorf("TSIG key %s not found", keyID))
		}
		return diag.FromErr(fmt.Errorf("couldn't fetch TSIG key %s: %w", keyID, err))
	}

	d.SetId(tsigKey.ID)

	if err := d.Set("name", tsigKey.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting TSIG key name: %w", err))
	}
	if err := d.Set("algorithm", tsigKey.Algorithm); err != nil {
		return diag.FromErr(fmt.Errorf("error setting TSIG key algorithm: %w", err))
	}
	if err := d.Set("key", tsigKey.Key); err != nil {
		return diag.FromErr(fmt.Errorf("error setting TSIG key material: %w", err))
	}

	return nil
}
