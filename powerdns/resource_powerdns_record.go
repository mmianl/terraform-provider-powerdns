package powerdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &recordResource{}
	_ resource.ResourceWithConfigure   = &recordResource{}
	_ resource.ResourceWithImportState = &recordResource{}
)

func newRecordResource() resource.Resource { return &recordResource{} }

type recordResource struct {
	clients *ProviderClients
}

type recordModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Disabled types.Bool   `tfsdk:"disabled"`
	Records  types.Set    `tfsdk:"records"`
	Comments types.List   `tfsdk:"comments"`
	SetPtr   types.Bool   `tfsdk:"set_ptr"`
}

func (r *recordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *recordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PowerDNS RRset.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The record name and type joined by `:::`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone": schema.StringAttribute{
				Description:   "The name of the zone, with a trailing dot. Changing it forces a new record.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateZoneName)},
			},
			"name": schema.StringAttribute{
				Description:   "The record name, a fully qualified domain name with a trailing dot. Changing it forces a new record.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateFQDN)},
			},
			"type": schema.StringAttribute{
				Description:   "The record type, such as `A` or `CNAME`. Changing it forces a new record.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ttl": schema.Int64Attribute{
				Description: "The time to live of the records, in seconds.",
				Required:    true,
			},
			"disabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether all records in this RRset are disabled in PowerDNS.",
			},
			"records": schema.SetAttribute{
				Description: "The record contents, one per element.",
				Required:    true,
				ElementType: types.StringType,
			},
			"comments": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Validators: []validator.List{
					listvalidator.ValueStringsAre(validateWith(validateRRSetComment)),
				},
				Description: "Ordered list of RRset comments stored in PowerDNS.",
			},
			"set_ptr": schema.BoolAttribute{
				Optional:           true,
				PlanModifiers:      []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				DeprecationMessage: "PowerDNS removed API support for automatic PTR record creation in Authoritative Server 4.4.0; setting this has no effect. Manage the PTR record explicitly with the powerdns_ptr_record resource instead.",
				Description:        "Deprecated and non-functional: PowerDNS has not honored this flag since Authoritative Server 4.4.0. Use the powerdns_ptr_record resource instead.",
			},
		},
	}
}

func (r *recordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// upsert writes the RRset. config is the raw configuration, state the prior
// state (nil on create).
func (r *recordResource) upsert(ctx context.Context, config, plan recordModel, state *recordModel, diags *diag.Diagnostics) (string, bool) {
	zone := plan.Zone.ValueString()
	name := plan.Name.ValueString()
	typ := plan.Type.ValueString()
	ttl := int(plan.TTL.ValueInt64())

	if strings.EqualFold(typ, "SOA") {
		diags.AddError("Unsupported record type", "SOA records cannot be managed with powerdns_record; use the powerdns_record_soa resource instead")
		return "", false
	}

	disabledConfigured := !config.Disabled.IsNull()
	disabled := configuredRRSetDisabledValue(disabledConfigured, plan.Disabled.ValueBool())
	recList := stringsFromSet(ctx, plan.Records, diags)
	if diags.HasError() {
		return "", false
	}
	setPtr := plan.SetPtr.ValueBool()

	// Basic validation for records content.
	for _, content := range recList {
		if strings.TrimSpace(content) == "" {
			tflog.Warn(ctx, "One or more values in 'records' are empty strings")
			break
		}
	}
	if len(recList) == 0 {
		diags.AddError("Invalid records", "'records' must not be empty")
		return "", false
	}

	rrSet := ResourceRecordSet{
		Name: name,
		Type: typ,
		TTL:  ttl,
	}

	disabledByContent := map[string]bool{}
	if state != nil {
		disabledChanged := !plan.Disabled.Equal(state.Disabled)
		if shouldPreserveRecordDisabledFlags(true, disabledConfigured, disabledChanged) {
			existingRRSet, err := r.clients.PDNS.GetRecordSetByID(ctx, zone, state.ID.ValueString())
			if err != nil {
				diags.AddError("Failed to fetch existing PowerDNS Record", err.Error())
				return "", false
			}
			disabledByContent = rrSetDisabledByContent(recordsFromRRSet(existingRRSet))
		}
	}

	records := make([]Record, 0, len(recList))
	for _, content := range recList {
		records = append(records, Record{
			Name:     rrSet.Name,
			Type:     rrSet.Type,
			TTL:      ttl,
			Content:  content,
			Disabled: recordDisabledValue(disabledByContent, content, disabled),
			SetPtr:   setPtr,
		})
	}
	rrSet.Records = records

	// Comments are only sent when configured; otherwise PowerDNS keeps them.
	if !config.Comments.IsNull() && !plan.Comments.IsUnknown() {
		comments := make([]Comment, 0)
		for _, c := range stringsFromList(ctx, plan.Comments, diags) {
			comments = append(comments, Comment{Content: c})
		}
		rrSet.Comments = &comments
	}

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	ctx = tflog.SetField(ctx, "type", typ)
	tflog.Debug(ctx, "Creating PowerDNS record set")

	recID, err := r.clients.PDNS.ReplaceRecordSet(ctx, zone, rrSet)
	if err != nil {
		diags.AddError("Failed to create PowerDNS Record", err.Error())
		return "", false
	}
	tflog.Info(ctx, "Created PowerDNS Record", map[string]any{"id": recID})
	return recID, true
}

// refresh loads the RRset into m. It reports false when it is gone.
func (r *recordResource) refresh(ctx context.Context, m *recordModel, diags *diag.Diagnostics) bool {
	zone := m.Zone.ValueString()
	id := m.ID.ValueString()
	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "record_id", id)
	tflog.Debug(ctx, "Reading PowerDNS Record")

	rrSet, err := r.clients.PDNS.GetRecordSetByID(ctx, zone, id)
	if err != nil {
		// The whole zone can disappear out of band, which leaves the record
		// gone as well rather than in an error state.
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing record from state")
			return false
		}
		diags.AddError("Couldn't fetch PowerDNS RRset details", err.Error())
		return false
	}
	records := recordsFromRRSet(rrSet)
	if len(records) == 0 {
		tflog.Warn(ctx, "PowerDNS Record not found; removing from state")
		return false
	}

	recs := make([]string, 0, len(records))
	for _, rec := range records {
		recs = append(recs, rec.Content)
	}
	recSet, d := types.SetValueFrom(ctx, types.StringType, recs)
	diags.Append(d...)

	var comments []string
	if rrSet != nil {
		comments = flattenRRSetComments(rrSet.Comments)
	} else {
		comments = flattenRRSetComments(nil)
	}
	commentList, d := types.ListValueFrom(ctx, types.StringType, comments)
	diags.Append(d...)
	if diags.HasError() {
		return false
	}

	m.Records = recSet
	m.Comments = commentList
	m.Disabled = types.BoolValue(rrSetDisabled(records))
	m.TTL = types.Int64Value(int64(records[0].TTL))
	m.Name = types.StringValue(records[0].Name)
	m.Type = types.StringValue(records[0].Type)
	return true
}

func (r *recordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config recordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := plan.Zone.ValueString()
	name := plan.Name.ValueString()
	typ := plan.Type.ValueString()

	if strings.EqualFold(typ, "SOA") {
		resp.Diagnostics.AddError("Unsupported record type", "SOA records cannot be managed with powerdns_record; use the powerdns_record_soa resource instead")
		return
	}

	// Held across the check and the write below so two resources for the same
	// zone/name/type created concurrently by this provider can't both observe
	// "does not exist" and both proceed.
	unlock := lockRecordCreate(zone, name, typ)
	defer unlock()

	if err := guardRecordOverwrite(ctx, r.clients.PDNS, "powerdns_record", zone, name, typ); err != nil {
		resp.Diagnostics.AddError("Record already exists", err.Error())
		return
	}

	id, ok := r.upsert(ctx, config, plan, nil, &resp.Diagnostics)
	if !ok {
		return
	}
	plan.ID = types.StringValue(id)

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("PowerDNS Record not found after creation", id)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.refresh(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *recordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, state recordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, ok := r.upsert(ctx, config, plan, &state, &resp.Diagnostics)
	if !ok {
		return
	}
	plan.ID = types.StringValue(id)

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("PowerDNS Record not found after update", id)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state recordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := state.Zone.ValueString()
	id := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "record_id", id)
	tflog.Debug(ctx, "Deleting PowerDNS Record")

	if err := r.clients.PDNS.DeleteRecordSetByID(ctx, zone, id); err != nil {
		resp.Diagnostics.AddError("Error deleting PowerDNS Record", err.Error())
		return
	}
	tflog.Info(ctx, "Deleted PowerDNS Record")
}

// importIDParts decodes the JSON import identifier {"zone": ..., "id": ...}.
func importIDParts(raw string) (zone string, id string, err error) {
	var data map[string]string
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", "", err
	}
	zone, ok := data["zone"]
	if !ok {
		return "", "", fmt.Errorf("missing zone name in input data")
	}
	id, ok = data["id"]
	if !ok {
		return "", "", fmt.Errorf("missing record id in input data")
	}
	return zone, id, nil
}

func (r *recordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing PowerDNS Record", map[string]any{"id": req.ID})

	zone, id, err := importIDParts(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
}

func configuredRRSetDisabledValue(disabledConfigured bool, disabled bool) bool {
	if !disabledConfigured {
		return false
	}

	return disabled
}

func shouldPreserveRecordDisabledFlags(hasExistingID bool, disabledConfigured bool, disabledChanged bool) bool {
	return hasExistingID && !disabledConfigured && !disabledChanged
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
