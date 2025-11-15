package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePDNSRecord() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSRecordCreate,
		ReadContext:   resourcePDNSRecordRead,
		DeleteContext: resourcePDNSRecordDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSRecordImport,
		},

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"ttl": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"records": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Required: true,
				ForceNew: true,
				Set:      schema.HashString,
			},
			"set_ptr": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Description: "For A and AAAA records, if true, create corresponding PTR.",
			},
		},
	}
}

func resourcePDNSRecordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	name := d.Get("name").(string)
	typ := d.Get("type").(string)
	ttl := d.Get("ttl").(int)
	recList := d.Get("records").(*schema.Set).List()

	setPtr := false
	if v, ok := d.GetOk("set_ptr"); ok {
		setPtr = v.(bool)
	}

	// Basic validation for records content (sets don't support ValidateFunc per element).
	for _, raw := range recList {
		if strings.TrimSpace(raw.(string)) == "" {
			tflog.Warn(ctx, "One or more values in 'records' are empty strings")
			break
		}
	}
	if len(recList) == 0 {
		return diag.FromErr(fmt.Errorf("'records' must not be empty"))
	}

	rrSet := ResourceRecordSet{
		Name: name,
		Type: typ,
		TTL:  ttl,
	}

	records := make([]Record, 0, len(recList))
	for _, rc := range recList {
		records = append(records, Record{
			Name:    rrSet.Name,
			Type:    rrSet.Type,
			TTL:     ttl,
			Content: rc.(string),
			SetPtr:  setPtr,
		})
	}
	rrSet.Records = records

	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "name", name)
	tflog.SetField(ctx, "type", typ)
	tflog.Debug(ctx, "Creating PowerDNS record set")

	recID, err := client.PDNS.ReplaceRecordSet(ctx, zone, rrSet)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create PowerDNS Record: %w", err))
	}

	d.SetId(recID)
	tflog.Info(ctx, "Created PowerDNS Record", map[string]any{"id": recID})

	return resourcePDNSRecordRead(ctx, d, meta)
}

func resourcePDNSRecordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "record_id", d.Id())
	tflog.Debug(ctx, "Reading PowerDNS Record")

	records, err := client.PDNS.ListRecordsByID(ctx, zone, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS Record: %w", err))
	}

	if len(records) == 0 {
		// rrset no longer exists; clear state
		tflog.Warn(ctx, "PowerDNS Record not found; removing from state")
		d.SetId("")
		return nil
	}

	recs := make([]string, 0, len(records))
	for _, r := range records {
		recs = append(recs, r.Content)
	}

	if err := d.Set("records", recs); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Records: %w", err))
	}
	if err := d.Set("ttl", records[0].TTL); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS TTL: %w", err))
	}
	if err := d.Set("name", records[0].Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Name: %w", err))
	}
	if err := d.Set("type", records[0].Type); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Type: %w", err))
	}

	return nil
}

func resourcePDNSRecordDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	tflog.SetField(ctx, "zone", zone)
	tflog.SetField(ctx, "record_id", d.Id())
	tflog.Debug(ctx, "Deleting PowerDNS Record")

	if err := client.PDNS.DeleteRecordSetByID(ctx, zone, d.Id()); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS Record: %w", err))
	}

	tflog.Info(ctx, "Deleted PowerDNS Record")
	return nil
}

// NOTE: Exists handlers are deprecated in SDKv2. Read should clear state when the object is missing.

func resourcePDNSRecordImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*ProviderClients)

	tflog.Info(ctx, "Importing PowerDNS Record", map[string]any{"id": d.Id()})

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

	tflog.Debug(ctx, "Fetching record for import", map[string]any{
		"zone": zoneName, "recordID": recordID,
	})

	records, err := client.PDNS.ListRecordsByID(ctx, zoneName, recordID)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch PowerDNS Record: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("rrset has no records to import")
	}

	recs := make([]string, 0, len(records))
	for _, r := range records {
		recs = append(recs, r.Content)
	}

	if err := d.Set("zone", zoneName); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS Zone: %w", err)
	}
	if err := d.Set("name", records[0].Name); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS Name: %w", err)
	}
	if err := d.Set("ttl", records[0].TTL); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS TTL: %w", err)
	}
	if err := d.Set("type", records[0].Type); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS Type: %w", err)
	}
	if err := d.Set("records", recs); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS Records: %w", err)
	}

	d.SetId(recordID)
	return []*schema.ResourceData{d}, nil
}
