package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSZoneCryptokeys() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSZoneCryptokeysRead,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateZoneName,
				Description:  "The zone whose DNSSEC keys should be retrieved.",
			},
			"cryptokeys": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The numeric ID PowerDNS assigned to this key",
						},
						"keytype": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The role of the key: ksk, zsk or csk",
						},
						"active": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the key is used for signing",
						},
						"published": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the DNSKEY record is published in the zone",
						},
						"algorithm": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The signing algorithm of the key",
						},
						"bits": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The key size in bits",
						},
						"dnskey": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The DNSKEY record for this key",
						},
						"ds": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "The DS records for this key",
						},
						"cds": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "The DS records for this key, filtered by CDS publication settings",
						},
					},
				},
				Description: "All DNSSEC keys configured for the zone",
			},
		},
	}
}

func dataSourcePDNSZoneCryptokeysRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	ctx = tflog.SetField(ctx, "zone_name", zone)
	tflog.Info(ctx, "Reading zone cryptokeys data source")

	cryptokeys, err := client.PDNS.ListCryptokeys(ctx, zone)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return diag.FromErr(fmt.Errorf("zone %s not found", zone))
		}
		return diag.FromErr(fmt.Errorf("couldn't fetch cryptokeys for zone %s: %w", zone, err))
	}

	d.SetId(zone)

	keys := make([]map[string]interface{}, 0, len(cryptokeys))
	for _, c := range cryptokeys {
		keys = append(keys, map[string]interface{}{
			"key_id":    c.ID,
			"keytype":   c.KeyType,
			"active":    c.Active,
			"published": c.Published,
			"algorithm": c.Algorithm,
			"bits":      c.Bits,
			"dnskey":    c.DNSKey,
			"ds":        c.DS,
			"cds":       c.CDS,
		})
	}

	if err := d.Set("cryptokeys", keys); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone cryptokeys: %w", err))
	}

	tflog.Info(ctx, "Successfully retrieved zone cryptokeys", map[string]interface{}{
		"key_count": len(keys),
	})
	return nil
}
