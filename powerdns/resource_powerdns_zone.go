package powerdns

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &zoneResource{}
	_ resource.ResourceWithConfigure   = &zoneResource{}
	_ resource.ResourceWithImportState = &zoneResource{}
)

func newZoneResource() resource.Resource { return &zoneResource{} }

type zoneResource struct {
	clients *ProviderClients
}

type zoneModel struct {
	ID               types.String        `tfsdk:"id"`
	Name             types.String        `tfsdk:"name"`
	Kind             semanticStringValue `tfsdk:"kind"`
	Account          types.String        `tfsdk:"account"`
	Catalog          types.String        `tfsdk:"catalog"`
	Masters          types.Set           `tfsdk:"masters"`
	SoaEditAPI       types.String        `tfsdk:"soa_edit_api"`
	SoaEdit          types.String        `tfsdk:"soa_edit"`
	DNSSec           types.Bool          `tfsdk:"dnssec"`
	APIRectify       types.Bool          `tfsdk:"api_rectify"`
	Serial           types.Int64         `tfsdk:"serial"`
	NotifiedSerial   types.Int64         `tfsdk:"notified_serial"`
	EditedSerial     types.Int64         `tfsdk:"edited_serial"`
	LastCheck        types.Int64         `tfsdk:"last_check"`
	MasterTsigKeyIDs types.Set           `tfsdk:"master_tsig_key_ids"`
	SlaveTsigKeyIDs  types.Set           `tfsdk:"slave_tsig_key_ids"`
}

// NormalizeTSIGKeyID accepts a bare name or the dotted ID returned by PowerDNS.
func NormalizeTSIGKeyID(keyID string) string {
	if keyID == "" || strings.HasSuffix(keyID, ".") {
		return keyID
	}
	return keyID + "."
}

func (r *zoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func zoneTSIGReferencesAttribute(description string, maxItems int) schema.SetAttribute {
	attr := schema.SetAttribute{
		Optional:      true,
		Computed:      true,
		ElementType:   types.StringType,
		PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
		Description:   description + " Bare names are normalized to dotted IDs; set [] to clear existing references.",
	}
	if maxItems > 0 {
		attr.Validators = []validator.Set{setvalidator.SizeAtMost(maxItems)}
	}
	return attr
}

func (r *zoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PowerDNS zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The zone name, with a trailing dot.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description:   "The zone name. It must end with a trailing dot. Changing it forces a new zone.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateZoneName)},
			},
			"kind": schema.StringAttribute{
				Description: "The zone kind: `Native`, `Master`, `Slave`, `Producer` or `Consumer`. Case does not matter.",
				Required:    true,
				CustomType:  caseInsensitiveStringType,
			},
			"account": schema.StringAttribute{
				Description: "The account the zone belongs to. Defaults to `admin`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("admin"),
				Validators:  []validator.String{stringvalidator.LengthBetween(0, 40)},
			},
			"catalog": schema.StringAttribute{
				Description: "The catalog zone this zone is a member of.",
				Optional:    true,
				Validators:  []validator.String{validateWith(ValidateZoneName)},
			},
			"masters": schema.SetAttribute{
				Description: "IP addresses of the primary servers, for a `Slave` zone.",
				Optional:    true,
				ElementType: masterAddressType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(validateWith(ValidateMasterAddress)),
				},
			},
			"soa_edit_api": schema.StringAttribute{
				Description:   "The SOA-EDIT-API setting, which controls how the serial changes on API edits. Read from the server when not set.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"soa_edit": schema.StringAttribute{
				Description:   "The SOA-EDIT setting, which controls how the serial is changed when answering. Read from the server when not set.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dnssec": schema.BoolAttribute{
				Description:   "Whether DNSSEC is enabled for the zone. Read from the server when not set.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"api_rectify": schema.BoolAttribute{
				Description:   "Whether PowerDNS rectifies the zone after each API change. Read from the server when not set.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"serial":          schema.Int64Attribute{Computed: true, Description: "The current SOA serial of the zone."},
			"notified_serial": schema.Int64Attribute{Computed: true, Description: "The last serial a NOTIFY was sent for."},
			"edited_serial":   schema.Int64Attribute{Computed: true, Description: "The SOA serial after the SOA-EDIT-API rules were applied."},
			"last_check":      schema.Int64Attribute{Computed: true, Description: "The time of the last check against the primary server, as a Unix timestamp."},
			"master_tsig_key_ids": zoneTSIGReferencesAttribute(
				"TSIG key IDs allowed to authenticate AXFR requests for this zone.", 0),
			"slave_tsig_key_ids": zoneTSIGReferencesAttribute(
				"TSIG key IDs used to sign requests to this zone's primary server.", 1),
		},
	}
}

func (r *zoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *zoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// tsigReferences returns the normalized, sorted, de-duplicated key IDs held by
// a set. A bare name and its dotted ID are one reference.
func tsigReferences(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	values := stringsFromSet(ctx, s, diags)
	return normalizeTSIGReferences(values)
}

func normalizeTSIGReferences(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = NormalizeTSIGKeyID(v)
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// tsigReferenceSet builds the value to store for a TSIG reference set. When
// the set already held in state or plan means the same references as the
// server reports, it is kept as written, so a bare name or a repeated
// spelling in the configuration does not show up as a change.
func tsigReferenceSet(ctx context.Context, current types.Set, fromAPI []string, diags *diag.Diagnostics) types.Set {
	api := normalizeTSIGReferences(fromAPI)
	if !current.IsNull() && !current.IsUnknown() {
		var held diag.Diagnostics
		if sameStrings(tsigReferences(ctx, current, &held), api) && !held.HasError() {
			return current
		}
	}
	set, d := types.SetValueFrom(ctx, types.StringType, api)
	diags.Append(d...)
	return set
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *zoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config zoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	masters := stringsFromSet(ctx, plan.Masters, &resp.Diagnostics)
	zoneInfo := ZoneInfo{
		Name:             plan.Name.ValueString(),
		Kind:             plan.Kind.ValueString(),
		Catalog:          plan.Catalog.ValueString(),
		Account:          plan.Account.ValueString(),
		Nameservers:      []string{},
		SoaEdit:          plan.SoaEdit.ValueString(),
		SoaEditAPI:       plan.SoaEditAPI.ValueString(),
		DNSSec:           plan.DNSSec.ValueBool(),
		MasterTsigKeyIDs: tsigReferences(ctx, plan.MasterTsigKeyIDs, &resp.Diagnostics),
		SlaveTsigKeyIDs:  tsigReferences(ctx, plan.SlaveTsigKeyIDs, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// An explicit false must be sent, while an omitted Optional+Computed
	// boolean must preserve the server default.
	if !config.APIRectify.IsNull() && !config.APIRectify.IsUnknown() {
		apiRectify := config.APIRectify.ValueBool()
		zoneInfo.APIRectify = &apiRectify
	}

	if len(masters) != 0 {
		if !strings.EqualFold(zoneInfo.Kind, "Slave") {
			resp.Diagnostics.AddError("Invalid masters", "masters attribute is supported only for Slave kind")
			return
		}
		zoneInfo.Masters = masters
	}

	ctx = tflog.SetField(ctx, "zone_name", zoneInfo.Name)
	ctx = tflog.SetField(ctx, "zone_kind", zoneInfo.Kind)
	tflog.Debug(ctx, "Creating PowerDNS Zone")

	created, err := r.clients.PDNS.CreateZone(ctx, zoneInfo)
	if err != nil {
		resp.Diagnostics.AddError("Error creating PowerDNS Zone", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	tflog.Info(ctx, "Created PowerDNS Zone", map[string]any{"id": created.ID})

	found := r.refresh(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.Diagnostics.AddError("Zone not found after creation", fmt.Sprintf("zone %s disappeared right after it was created", created.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// refresh loads the zone from PowerDNS into m. It reports false when the zone
// no longer exists.
func (r *zoneResource) refresh(ctx context.Context, m *zoneModel, diags *diag.Diagnostics) bool {
	zoneInfo, err := r.clients.PDNS.GetZone(ctx, m.ID.ValueString())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing from state")
			return false
		}
		diags.AddError("Couldn't fetch PowerDNS Zone", err.Error())
		return false
	}
	if zoneInfo.Name == "" {
		tflog.Warn(ctx, "Zone not found; removing from state")
		return false
	}

	m.Name = types.StringValue(zoneInfo.Name)
	m.Kind = newSemanticString(caseInsensitiveStringType, zoneInfo.Kind)
	m.Account = types.StringValue(zoneInfo.Account)
	m.Catalog = stringOrNull(zoneInfo.Catalog)
	m.SoaEditAPI = types.StringValue(zoneInfo.SoaEditAPI)
	m.SoaEdit = types.StringValue(zoneInfo.SoaEdit)
	m.DNSSec = types.BoolValue(zoneInfo.DNSSec)
	if zoneInfo.APIRectify != nil {
		m.APIRectify = types.BoolValue(*zoneInfo.APIRectify)
	} else if m.APIRectify.IsUnknown() {
		m.APIRectify = types.BoolNull()
	}
	m.Serial = types.Int64Value(zoneInfo.Serial)
	m.NotifiedSerial = types.Int64Value(zoneInfo.NotifiedSerial)
	m.EditedSerial = types.Int64Value(zoneInfo.EditedSerial)
	m.LastCheck = types.Int64Value(zoneInfo.LastCheck)

	m.MasterTsigKeyIDs = tsigReferenceSet(ctx, m.MasterTsigKeyIDs, zoneInfo.MasterTsigKeyIDs, diags)
	m.SlaveTsigKeyIDs = tsigReferenceSet(ctx, m.SlaveTsigKeyIDs, zoneInfo.SlaveTsigKeyIDs, diags)
	if diags.HasError() {
		return false
	}

	if strings.EqualFold(zoneInfo.Kind, "Slave") && (len(zoneInfo.Masters) != 0 || !m.Masters.IsNull()) {
		set, d := types.SetValueFrom(ctx, masterAddressType, zoneInfo.Masters)
		diags.Append(d...)
		if d.HasError() {
			return false
		}
		m.Masters = set
	}
	return true
}

func (r *zoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "zone_id", state.ID.ValueString())
	tflog.Debug(ctx, "Reading PowerDNS Zone")

	if !r.refresh(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *zoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state zoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	ctx = tflog.SetField(ctx, "zone_id", plan.ID.ValueString())
	tflog.Debug(ctx, "Updating PowerDNS Zone")

	masters := stringsFromSet(ctx, plan.Masters, &resp.Diagnostics)
	planMaster := tsigReferences(ctx, plan.MasterTsigKeyIDs, &resp.Diagnostics)
	planSlave := tsigReferences(ctx, plan.SlaveTsigKeyIDs, &resp.Diagnostics)
	stateMaster := tsigReferences(ctx, state.MasterTsigKeyIDs, &resp.Diagnostics)
	stateSlave := tsigReferences(ctx, state.SlaveTsigKeyIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneInfo := ZoneInfoUpd{
		Name:       plan.Name.ValueString(),
		Kind:       plan.Kind.ValueString(),
		Catalog:    plan.Catalog.ValueString(),
		Account:    plan.Account.ValueString(),
		SoaEditAPI: plan.SoaEditAPI.ValueString(),
		Masters:    masters,
	}
	// Only send these optional settings when changed, including false and
	// empty values, so unrelated updates preserve server-managed settings.
	if !plan.SoaEdit.Equal(state.SoaEdit) {
		value := plan.SoaEdit.ValueString()
		zoneInfo.SoaEdit = &value
	}
	if !plan.DNSSec.Equal(state.DNSSec) {
		value := plan.DNSSec.ValueBool()
		zoneInfo.DNSSec = &value
	}
	if !plan.APIRectify.Equal(state.APIRectify) {
		value := plan.APIRectify.ValueBool()
		zoneInfo.APIRectify = &value
	}
	// Pointer fields distinguish clearing an explicitly managed set from
	// omitting it during an unrelated zone update.
	if !sameStrings(planMaster, stateMaster) {
		zoneInfo.MasterTsigKeyIDs = &planMaster
	}
	if !sameStrings(planSlave, stateSlave) {
		zoneInfo.SlaveTsigKeyIDs = &planSlave
	}

	if err := r.clients.PDNS.UpdateZone(ctx, plan.ID.ValueString(), zoneInfo); err != nil {
		resp.Diagnostics.AddError("Error updating PowerDNS Zone", err.Error())
		return
	}

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Zone not found after update", plan.ID.ValueString())
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "zone_id", state.ID.ValueString())
	tflog.Debug(ctx, "Deleting PowerDNS Zone")

	if err := r.clients.PDNS.DeleteZone(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting PowerDNS Zone", err.Error())
		return
	}
	tflog.Info(ctx, "Deleted PowerDNS Zone")
}
