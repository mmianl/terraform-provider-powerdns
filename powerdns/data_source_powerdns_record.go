package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSRecord() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePDNSRecordRead,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "The name of the zone containing the record. Must be a fully qualified domain name ending with a trailing dot.",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "The fully qualified record name ending with a trailing dot.",
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The RRset type to look up, for example A, AAAA, MX, or TXT.",
			},
			"ttl": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The TTL of the RRset in seconds.",
			},
			"disabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether all records in the RRset are disabled in PowerDNS.",
			},
			"records": {
				Type:        schema.TypeSet,
				Computed:    true,
				Set:         schema.HashString,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Set of record contents in the RRset.",
			},
			"comments": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Ordered list of RRset comments stored in PowerDNS.",
			},
		},
	}
}

func dataSourcePDNSRecordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	name := d.Get("name").(string)
	typ := d.Get("type").(string)

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	ctx = tflog.SetField(ctx, "type", typ)
	tflog.Info(ctx, "Reading record data source")

	records, err := client.PDNS.ListRecordsInRRSet(ctx, zone, name, typ)
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS record %s %s in zone %s: %w", name, typ, zone, err))
	}
	if len(records) == 0 {
		return diag.FromErr(fmt.Errorf("record %s %s not found in zone %s", name, typ, zone))
	}

	recordContents := make([]string, 0, len(records))
	for _, record := range records {
		recordContents = append(recordContents, record.Content)
	}

	rrSet, err := client.PDNS.GetRecordSetByID(ctx, zone, records[0].ID())
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS RRset details for %s %s in zone %s: %w", name, typ, zone, err))
	}

	d.SetId(records[0].ID())

	if err := d.Set("zone", zone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting zone: %w", err))
	}
	if err := d.Set("name", records[0].Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting name: %w", err))
	}
	if err := d.Set("type", records[0].Type); err != nil {
		return diag.FromErr(fmt.Errorf("error setting type: %w", err))
	}
	if err := d.Set("ttl", records[0].TTL); err != nil {
		return diag.FromErr(fmt.Errorf("error setting ttl: %w", err))
	}
	if err := d.Set("disabled", rrSetDisabled(records)); err != nil {
		return diag.FromErr(fmt.Errorf("error setting disabled: %w", err))
	}
	if err := d.Set("records", recordContents); err != nil {
		return diag.FromErr(fmt.Errorf("error setting records: %w", err))
	}
	if err := d.Set("comments", flattenRRSetComments(commentsFromRRSet(rrSet))); err != nil {
		return diag.FromErr(fmt.Errorf("error setting comments: %w", err))
	}

	tflog.Info(ctx, "Successfully retrieved PowerDNS record", map[string]any{
		"record_count": len(recordContents),
	})
	return nil
}

func commentsFromRRSet(rrSet *ResourceRecordSet) *[]Comment {
	if rrSet == nil {
		return nil
	}

	return rrSet.Comments
}
