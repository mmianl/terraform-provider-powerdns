package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePDNSRecord() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSRecordCreate,
		ReadContext:   resourcePDNSRecordRead,
		UpdateContext: resourcePDNSRecordUpdate,
		DeleteContext: resourcePDNSRecordDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSRecordImport,
		},

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateZoneName,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateFQDN,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"ttl": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"disabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether all records in this RRset are disabled in PowerDNS.",
			},
			"records": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Required: true,
				Set:      schema.HashString,
			},
			"comments": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validateRRSetComment,
				},
				Optional:    true,
				Computed:    true,
				Description: "Ordered list of RRset comments stored in PowerDNS.",
			},
			"set_ptr": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Deprecated:  "PowerDNS removed API support for automatic PTR record creation in Authoritative Server 4.4.0; setting this has no effect. Manage the PTR record explicitly with the powerdns_ptr_record resource instead.",
				Description: "Deprecated and non-functional: PowerDNS has not honored this flag since Authoritative Server 4.4.0. Use the powerdns_ptr_record resource instead.",
			},
		},
	}
}

func resourcePDNSRecordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourcePDNSRecordUpsert(ctx, d, meta)
}

func resourcePDNSRecordUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourcePDNSRecordUpsert(ctx, d, meta)
}

func resourcePDNSRecordUpsert(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	name := d.Get("name").(string)
	typ := d.Get("type").(string)
	ttl := d.Get("ttl").(int)
	disabled := configuredRRSetDisabledValue(d.GetRawConfig(), d.Get("disabled").(bool))
	recList := d.Get("records").(*schema.Set).List()
	if strings.EqualFold(typ, "SOA") {
		return diag.FromErr(fmt.Errorf("SOA records cannot be managed with powerdns_record; use the powerdns_record_soa resource instead"))
	}

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

	disabledByContent := map[string]bool{}
	if shouldPreserveRecordDisabledFlags(d.Id() != "", rrSetDisabledConfigured(d.GetRawConfig()), d.HasChange("disabled")) {
		existingRRSet, err := client.PDNS.GetRecordSetByID(ctx, zone, d.Id())
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to fetch existing PowerDNS Record: %w", err))
		}

		disabledByContent = rrSetDisabledByContent(recordsFromRRSet(existingRRSet))
	}

	records := make([]Record, 0, len(recList))
	for _, rc := range recList {
		content := rc.(string)
		recordDisabled := recordDisabledValue(disabledByContent, content, disabled)

		records = append(records, Record{
			Name:     rrSet.Name,
			Type:     rrSet.Type,
			TTL:      ttl,
			Content:  content,
			Disabled: recordDisabled,
			SetPtr:   setPtr,
		})
	}
	rrSet.Records = records
	rrSet.Comments = configuredRRSetComments(d)

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	ctx = tflog.SetField(ctx, "type", typ)
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
	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "record_id", d.Id())
	tflog.Debug(ctx, "Reading PowerDNS Record")

	rrSet, err := client.PDNS.GetRecordSetByID(ctx, zone, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS RRset details: %w", err))
	}
	records := recordsFromRRSet(rrSet)

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

	comments := flattenRRSetComments(nil)
	if rrSet != nil {
		comments = flattenRRSetComments(rrSet.Comments)
	}

	if err := d.Set("records", recs); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Records: %w", err))
	}
	if err := d.Set("comments", comments); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Comments: %w", err))
	}
	if err := d.Set("disabled", rrSetDisabled(records)); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Disabled: %w", err))
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
	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "record_id", d.Id())
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

	rrSet, err := client.PDNS.GetRecordSetByID(ctx, zoneName, recordID)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch PowerDNS RRset details: %w", err)
	}
	records := recordsFromRRSet(rrSet)
	if len(records) == 0 {
		return nil, fmt.Errorf("rrset has no records to import")
	}

	recs := make([]string, 0, len(records))
	for _, r := range records {
		recs = append(recs, r.Content)
	}

	comments := flattenRRSetComments(nil)
	if rrSet != nil {
		comments = flattenRRSetComments(rrSet.Comments)
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
	if err := d.Set("comments", comments); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS Comments: %w", err)
	}
	if err := d.Set("disabled", rrSetDisabled(records)); err != nil {
		return nil, fmt.Errorf("error setting PowerDNS Disabled: %w", err)
	}

	d.SetId(recordID)
	return []*schema.ResourceData{d}, nil
}

func configuredRRSetComments(d *schema.ResourceData) *[]Comment {
	if d.HasChange("comments") {
		_, newComments := d.GetChange("comments")
		return expandRRSetCommentsValue(newComments)
	}

	rawConfig := d.GetRawConfig()
	if !rawConfig.IsNull() && rawConfig.IsKnown() {
		rawComments := rawConfig.GetAttr("comments")
		if rawComments.IsKnown() && !rawComments.IsNull() {
			return expandRRSetCommentsValue(d.Get("comments"))
		}
	}

	return nil
}

func rrSetDisabledConfigured(rawConfig cty.Value) bool {
	if rawConfig.IsNull() || !rawConfig.IsKnown() {
		return false
	}

	rawDisabled := rawConfig.GetAttr("disabled")
	return rawDisabled.IsKnown() && !rawDisabled.IsNull()
}

func shouldPreserveRecordDisabledFlags(hasExistingID bool, disabledConfigured bool, disabledChanged bool) bool {
	return hasExistingID && !disabledConfigured && !disabledChanged
}

func configuredRRSetDisabledValue(rawConfig cty.Value, disabled bool) bool {
	if !rrSetDisabledConfigured(rawConfig) {
		return false
	}

	return disabled
}

func expandRRSetComments(rawComments []interface{}) []Comment {
	comments := make([]Comment, 0, len(rawComments))
	for _, rawComment := range rawComments {
		content, ok := rawComment.(string)
		if !ok {
			continue
		}

		comments = append(comments, Comment{Content: content})
	}

	return comments
}

func expandRRSetCommentsValue(rawComments any) *[]Comment {
	if rawComments == nil {
		comments := []Comment{}
		return &comments
	}

	switch comments := rawComments.(type) {
	case []interface{}:
		expanded := expandRRSetComments(comments)
		return &expanded
	case []string:
		raw := make([]interface{}, 0, len(comments))
		for _, comment := range comments {
			raw = append(raw, comment)
		}
		expanded := expandRRSetComments(raw)
		return &expanded
	default:
		emptyComments := []Comment{}
		return &emptyComments
	}
}

func flattenRRSetComments(comments *[]Comment) []string {
	if comments == nil {
		return []string{}
	}

	flattened := make([]string, 0, len(*comments))
	for _, comment := range *comments {
		flattened = append(flattened, comment.Content)
	}

	return flattened
}

func rrSetDisabled(records []Record) bool {
	if len(records) == 0 {
		return false
	}

	for _, record := range records {
		if !record.Disabled {
			return false
		}
	}

	return true
}

func rrSetDisabledByContent(records []Record) map[string]bool {
	disabledByContent := make(map[string]bool, len(records))
	for _, record := range records {
		disabledByContent[record.Content] = record.Disabled
	}

	return disabledByContent
}

func recordDisabledValue(disabledByContent map[string]bool, content string, defaultDisabled bool) bool {
	if disabled, ok := disabledByContent[content]; ok {
		return disabled
	}

	return defaultDisabled
}

func validateRRSetComment(v interface{}, k string) ([]string, []error) {
	comment, ok := v.(string)
	if !ok {
		return nil, []error{fmt.Errorf("%s must be a string", k)}
	}

	if strings.TrimSpace(comment) == "" {
		return nil, []error{fmt.Errorf("%s must not be empty or whitespace only", k)}
	}

	return nil, nil
}
