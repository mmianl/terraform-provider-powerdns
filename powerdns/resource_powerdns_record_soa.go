package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePDNSRecordSOA() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSRecordSOACreateOrUpdate,
		ReadContext:   resourcePDNSRecordSOARead,
		UpdateContext: resourcePDNSRecordSOACreateOrUpdate,
		DeleteContext: resourcePDNSRecordSOADelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSRecordSOAImport,
		},

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateFQDN,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateFQDN,
			},
			"ttl": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"mname": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "Primary nameserver (MNAME). Must be a fully qualified domain name ending with a trailing dot.",
			},
			"rname": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateFQDN,
				Description:  "Responsible person email in DNS format (RNAME). Must be a fully qualified domain name ending with a trailing dot.",
			},
			"serial": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "SOA serial number. If omitted or 0, PowerDNS manages it via soa_edit_api.",
			},
			"refresh": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Refresh interval in seconds.",
			},
			"retry": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Retry interval in seconds.",
			},
			"expire": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Expire time in seconds.",
			},
			"minimum": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Minimum TTL (negative caching) in seconds.",
			},
		},
	}
}

func buildSOAContent(d *schema.ResourceData) string {
	mname := d.Get("mname").(string)
	rname := d.Get("rname").(string)
	serial := d.Get("serial").(int)
	refresh := d.Get("refresh").(int)
	retry := d.Get("retry").(int)
	expire := d.Get("expire").(int)
	minimum := d.Get("minimum").(int)

	return fmt.Sprintf("%s %s %d %d %d %d %d", mname, rname, serial, refresh, retry, expire, minimum)
}

func parseSOAContent(content string) (mname, rname string, serial, refresh, retry, expire, minimum int, err error) {
	fields := strings.Fields(content)
	if len(fields) != 7 {
		err = fmt.Errorf("unexpected SOA content format: expected 7 fields, got %d", len(fields))
		return
	}

	mname = fields[0]
	rname = fields[1]

	nums := make([]int, 5)
	for i, f := range fields[2:] {
		nums[i], err = strconv.Atoi(f)
		if err != nil {
			err = fmt.Errorf("error parsing SOA field %d (%q): %w", i+2, f, err)
			return
		}
	}

	serial = nums[0]
	refresh = nums[1]
	retry = nums[2]
	expire = nums[3]
	minimum = nums[4]
	return
}

func resourcePDNSRecordSOACreateOrUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	name := d.Get("name").(string)
	ttl := d.Get("ttl").(int)
	content := buildSOAContent(d)

	rrSet := ResourceRecordSet{
		Name: name,
		Type: "SOA",
		TTL:  ttl,
		Records: []Record{
			{
				Name:    name,
				Type:    "SOA",
				TTL:     ttl,
				Content: content,
			},
		},
	}

	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "name", name)
	tflog.Debug(ctx, "Creating/updating PowerDNS SOA record")

	recID, err := client.PDNS.ReplaceRecordSet(ctx, zone, rrSet)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create/update PowerDNS SOA record: %w", err))
	}

	d.SetId(recID)
	tflog.Info(ctx, "Created/updated PowerDNS SOA record", map[string]any{"id": recID})

	return resourcePDNSRecordSOARead(ctx, d, meta)
}

func resourcePDNSRecordSOARead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "record_id", d.Id())
	tflog.Debug(ctx, "Reading PowerDNS SOA record")

	records, err := client.PDNS.ListRecordsByID(ctx, zone, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS SOA record: %w", err))
	}

	if len(records) == 0 {
		tflog.Warn(ctx, "PowerDNS SOA record not found; removing from state")
		d.SetId("")
		return nil
	}

	mname, rname, serial, refresh, retry, expire, minimum, err := parseSOAContent(records[0].Content)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error parsing SOA content: %w", err))
	}

	if err := d.Set("ttl", records[0].TTL); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", records[0].Name); err != nil {
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

	return nil
}

func resourcePDNSRecordSOADelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "record_id", d.Id())
	tflog.Debug(ctx, "Deleting PowerDNS SOA record")

	if err := client.PDNS.DeleteRecordSetByID(ctx, zone, d.Id()); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS SOA record: %w", err))
	}

	tflog.Info(ctx, "Deleted PowerDNS SOA record")
	return nil
}

func resourcePDNSRecordSOAImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*ProviderClients)

	tflog.Info(ctx, "Importing PowerDNS SOA record", map[string]any{"id": d.Id()})

	var data map[string]string
	if err := json.Unmarshal([]byte(d.Id()), &data); err != nil {
		return nil, err
	}

	zoneName, ok := data["zone"]
	if !ok {
		return nil, fmt.Errorf("missing zone name in input data")
	}
	recordID, ok := data["id"]
	if !ok {
		return nil, fmt.Errorf("missing record id in input data")
	}

	tflog.Debug(ctx, "Fetching SOA record for import", map[string]any{
		"zone": zoneName, "recordID": recordID,
	})

	records, err := client.PDNS.ListRecordsByID(ctx, zoneName, recordID)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch PowerDNS SOA record: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("SOA record not found for import")
	}

	mname, rname, serial, refresh, retry, expire, minimum, err := parseSOAContent(records[0].Content)
	if err != nil {
		return nil, fmt.Errorf("error parsing SOA content: %w", err)
	}

	if err := d.Set("zone", zoneName); err != nil {
		return nil, err
	}
	if err := d.Set("name", records[0].Name); err != nil {
		return nil, err
	}
	if err := d.Set("ttl", records[0].TTL); err != nil {
		return nil, err
	}
	if err := d.Set("mname", mname); err != nil {
		return nil, err
	}
	if err := d.Set("rname", rname); err != nil {
		return nil, err
	}
	if err := d.Set("serial", serial); err != nil {
		return nil, err
	}
	if err := d.Set("refresh", refresh); err != nil {
		return nil, err
	}
	if err := d.Set("retry", retry); err != nil {
		return nil, err
	}
	if err := d.Set("expire", expire); err != nil {
		return nil, err
	}
	if err := d.Set("minimum", minimum); err != nil {
		return nil, err
	}

	d.SetId(recordID)
	return []*schema.ResourceData{d}, nil
}
