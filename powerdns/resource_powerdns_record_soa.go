package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &recordSOAResource{}
	_ resource.ResourceWithConfigure   = &recordSOAResource{}
	_ resource.ResourceWithImportState = &recordSOAResource{}
)

func newRecordSOAResource() resource.Resource { return &recordSOAResource{} }

type recordSOAResource struct {
	clients *ProviderClients
}

type recordSOAModel struct {
	ID       types.String       `tfsdk:"id"`
	Zone     types.String       `tfsdk:"zone"`
	Name     types.String       `tfsdk:"name"`
	TTL      types.Int64        `tfsdk:"ttl"`
	Disabled types.Bool         `tfsdk:"disabled"`
	Mname    types.String       `tfsdk:"mname"`
	Rname    types.String       `tfsdk:"rname"`
	Serial   semanticInt64Value `tfsdk:"serial"`
	Refresh  types.Int64        `tfsdk:"refresh"`
	Retry    types.Int64        `tfsdk:"retry"`
	Expire   types.Int64        `tfsdk:"expire"`
	Minimum  types.Int64        `tfsdk:"minimum"`
}

func (r *recordSOAResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record_soa"
}

func (r *recordSOAResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the SOA record of a PowerDNS zone.",
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
				Description:   "The SOA record name, usually the zone name. Changing it forces a new record.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateFQDN)},
			},
			"ttl": schema.Int64Attribute{
				Description: "The time to live of the record, in seconds.",
				Required:    true,
			},
			"disabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the SOA record is disabled in PowerDNS.",
			},
			"mname": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateFQDN)},
				Description: "Primary nameserver (MNAME). Must be a fully qualified domain name ending with a trailing dot.",
			},
			"rname": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateFQDN)},
				Description: "Responsible person email in DNS format (RNAME). Must be a fully qualified domain name ending with a trailing dot.",
			},
			"serial": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				// Configuring 0 means "let soa_edit_api manage this", so whatever
				// PowerDNS actually computes and stores is never treated as drift
				// against that 0 (see soaSerialType).
				//
				// An omitted serial is left unknown whenever the record changes,
				// because PowerDNS may advance it as part of the edit. It keeps
				// its value only when nothing else changes.
				CustomType:  soaSerialType,
				Description: "SOA serial number. If omitted or 0, PowerDNS manages it via soa_edit_api.",
			},
			"refresh": schema.Int64Attribute{
				Required:    true,
				Description: "Refresh interval in seconds.",
			},
			"retry": schema.Int64Attribute{
				Required:    true,
				Description: "Retry interval in seconds.",
			},
			"expire": schema.Int64Attribute{
				Required:    true,
				Description: "Expire time in seconds.",
			},
			"minimum": schema.Int64Attribute{
				Required:    true,
				Description: "Minimum TTL (negative caching) in seconds.",
			},
		},
	}
}

func (r *recordSOAResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func buildSOAContent(m recordSOAModel) string {
	return fmt.Sprintf("%s %s %d %d %d %d %d",
		m.Mname.ValueString(),
		m.Rname.ValueString(),
		m.Serial.ValueInt64(),
		m.Refresh.ValueInt64(),
		m.Retry.ValueInt64(),
		m.Expire.ValueInt64(),
		m.Minimum.ValueInt64(),
	)
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

// populate copies a stored SOA record into m.
func (m *recordSOAModel) populate(rec Record) error {
	mname, rname, serial, refresh, retry, expire, minimum, err := parseSOAContent(rec.Content)
	if err != nil {
		return err
	}
	m.Name = types.StringValue(rec.Name)
	m.TTL = types.Int64Value(int64(rec.TTL))
	m.Disabled = types.BoolValue(rec.Disabled)
	m.Mname = types.StringValue(mname)
	m.Rname = types.StringValue(rname)
	m.Serial = newSemanticInt64(soaSerialType, int64(serial))
	m.Refresh = types.Int64Value(int64(refresh))
	m.Retry = types.Int64Value(int64(retry))
	m.Expire = types.Int64Value(int64(expire))
	m.Minimum = types.Int64Value(int64(minimum))
	return nil
}

func (r *recordSOAResource) upsert(ctx context.Context, m *recordSOAModel, diags *diag.Diagnostics) {
	zone := m.Zone.ValueString()
	name := m.Name.ValueString()
	ttl := int(m.TTL.ValueInt64())

	rrSet := ResourceRecordSet{
		Name: name,
		Type: "SOA",
		TTL:  ttl,
		Records: []Record{
			{
				Name:     name,
				Type:     "SOA",
				TTL:      ttl,
				Content:  buildSOAContent(*m),
				Disabled: m.Disabled.ValueBool(),
			},
		},
	}

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	tflog.Debug(ctx, "Creating/updating PowerDNS SOA record")

	recID, err := r.clients.PDNS.ReplaceRecordSet(ctx, zone, rrSet)
	if err != nil {
		diags.AddError("Failed to create/update PowerDNS SOA record", err.Error())
		return
	}

	m.ID = types.StringValue(recID)
	tflog.Info(ctx, "Created/updated PowerDNS SOA record", map[string]any{"id": recID})

	found := r.refresh(ctx, m, diags)
	if !found && !diags.HasError() {
		diags.AddError("PowerDNS SOA record not found after write", recID)
	}
}

// refresh loads the SOA record into m. It reports false when it is gone.
func (r *recordSOAResource) refresh(ctx context.Context, m *recordSOAModel, diags *diag.Diagnostics) bool {
	zone := m.Zone.ValueString()
	id := m.ID.ValueString()
	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "record_id", id)
	tflog.Debug(ctx, "Reading PowerDNS SOA record")

	records, err := r.clients.PDNS.ListRecordsByID(ctx, zone, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing SOA record from state")
			return false
		}
		diags.AddError("Couldn't fetch PowerDNS SOA record", err.Error())
		return false
	}
	if len(records) == 0 {
		tflog.Warn(ctx, "PowerDNS SOA record not found; removing from state")
		return false
	}

	if err := m.populate(records[0]); err != nil {
		diags.AddError("Error parsing SOA content", err.Error())
		return false
	}
	return true
}

func (r *recordSOAResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan recordSOAModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		if !plan.ID.IsNull() && !plan.ID.IsUnknown() {
			keepCreated(ctx, req.Plan, &resp.State, plan.ID.ValueString(), &resp.Diagnostics)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recordSOAResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state recordSOAModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// With no serial configured, send the current one and let PowerDNS advance
	// it. Sending 0 would reset it on servers that do not manage serials.
	if plan.Serial.IsUnknown() {
		plan.Serial = state.Serial
	}
	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recordSOAResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recordSOAModel
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

func (r *recordSOAResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state recordSOAModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := state.Zone.ValueString()
	id := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "record_id", id)
	tflog.Debug(ctx, "Deleting PowerDNS SOA record")

	if err := r.clients.PDNS.DeleteRecordSetByID(ctx, zone, id); err != nil {
		resp.Diagnostics.AddError("Error deleting PowerDNS SOA record", err.Error())
		return
	}
	tflog.Info(ctx, "Deleted PowerDNS SOA record")
}

func (r *recordSOAResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing PowerDNS SOA record", map[string]any{"id": req.ID})

	zone, id, err := importIDParts(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
}
