package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSRecordSOA() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSRecordSOARead,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "The name of the zone containing the SOA record. Must be a fully qualified domain name ending with a trailing dot.",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "The name of the SOA record (usually the same as the zone name). Must be a fully qualified domain name ending with a trailing dot.",
			},
			"ttl": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The TTL of the SOA record.",
			},
			"disabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the SOA record is disabled.",
			},
			"mname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Primary nameserver (MNAME).",
			},
			"rname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Responsible person email in DNS format (RNAME).",
			},
			"serial": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "SOA serial number.",
			},
			"refresh": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Refresh interval in seconds.",
			},
			"retry": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Retry interval in seconds.",
			},
			"expire": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Expire time in seconds.",
			},
			"minimum": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Minimum TTL (negative caching) in seconds.",
			},
		},
	}
}

func dataSourcePDNSRecordSOARead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	name := d.Get("name").(string)

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	tflog.Info(ctx, "Reading SOA record data source")

	records, err := client.PDNS.ListRecordsInRRSet(ctx, zone, name, "SOA")
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch SOA record for %s in zone %s: %w", name, zone, err))
	}

	if len(records) == 0 {
		return diag.FromErr(fmt.Errorf("SOA record for %s not found in zone %s", name, zone))
	}

	mname, rname, serial, refresh, retry, expire, minimum, err := parseSOAContent(records[0].Content)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error parsing SOA content: %w", err))
	}

	d.SetId(records[0].ID())

	if err := d.Set("ttl", records[0].TTL); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("disabled", records[0].Disabled); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("mname", mname); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("rname", rname); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("serial", serial); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("refresh", refresh); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("retry", retry); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("expire", expire); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("minimum", minimum); err != nil {
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully retrieved SOA record")
	return nil
}
