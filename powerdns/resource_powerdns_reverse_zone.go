package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &reverseZoneResource{}
	_ resource.ResourceWithConfigure   = &reverseZoneResource{}
	_ resource.ResourceWithImportState = &reverseZoneResource{}
)

func newReverseZoneResource() resource.Resource { return &reverseZoneResource{} }

type reverseZoneResource struct {
	clients *ProviderClients
}

type reverseZoneModel struct {
	ID          types.String `tfsdk:"id"`
	CIDR        types.String `tfsdk:"cidr"`
	Kind        types.String `tfsdk:"kind"`
	Nameservers types.List   `tfsdk:"nameservers"`
	Name        types.String `tfsdk:"name"`
}

func (r *reverseZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_zone"
}

func (r *reverseZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PowerDNS reverse zone derived from a CIDR block.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The reverse zone name, with a trailing dot.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cidr": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateCIDR)},
				Description:   "The CIDR block for the reverse zone (e.g., '172.16.0.0/16')",
			},
			"kind": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf("Master", "Slave")},
				Description:   "The kind of zone (Master or Slave)",
			},
			"nameservers": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(validateWith(ValidateFQDN)),
				},
				Description: "List of nameservers for this zone. Each nameserver must be a fully qualified domain name ending with a trailing dot.",
			},
			"name": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "The computed zone name (e.g., '16.172.in-addr.arpa.')",
			},
		},
	}
}

func (r *reverseZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func expandStringList(configured []interface{}) []string {
	vs := make([]string, 0, len(configured))
	for _, v := range configured {
		vs = append(vs, v.(string))
	}
	return vs
}

func (r *reverseZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	zoneName := req.ID
	tflog.Info(ctx, "Importing reverse zone", map[string]any{"zone": zoneName})

	cidr, err := ParseReverseZoneName(zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Invalid reverse zone name", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), zoneName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), zoneName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cidr"), cidr)...)
}

// refresh loads the zone into m, reporting false when it is gone.
func (r *reverseZoneResource) refresh(ctx context.Context, m *reverseZoneModel, diags *diag.Diagnostics) bool {
	zoneName := m.ID.ValueString()

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Reading reverse zone")

	zone, err := r.clients.PDNS.GetZone(ctx, zoneName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing from state")
			return false
		}
		diags.AddError("Couldn't fetch zone", err.Error())
		return false
	}
	if zone.Name == "" {
		tflog.Warn(ctx, "Zone not found; removing from state")
		return false
	}

	tflog.Info(ctx, "Found reverse zone", map[string]any{"zone": zone.Name, "kind": zone.Kind})

	// Read nameservers from NS records
	nameservers, err := r.clients.PDNS.ListRecordsInRRSet(ctx, zoneName, zoneName, "NS")
	if err != nil {
		diags.AddError("Couldn't fetch zone nameservers", fmt.Sprintf("couldn't fetch zone %s nameservers from PowerDNS: %s", zoneName, err))
		return false
	}
	zoneNameservers := make([]string, 0, len(nameservers))
	for _, ns := range nameservers {
		zoneNameservers = append(zoneNameservers, ns.Content)
	}
	list, d := types.ListValueFrom(ctx, types.StringType, zoneNameservers)
	diags.Append(d...)
	if d.HasError() {
		return false
	}

	m.Name = types.StringValue(zone.Name)
	m.Kind = types.StringValue(zone.Kind)
	m.Nameservers = list
	return true
}

func (r *reverseZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan reverseZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cidr := plan.CIDR.ValueString()
	ctx = tflog.SetField(ctx, "cidr", cidr)
	tflog.Debug(ctx, "Creating reverse zone")

	zoneName, err := GetReverseZoneName(cidr)
	if err != nil {
		resp.Diagnostics.AddError("Failed to determine zone name", err.Error())
		return
	}
	tflog.Info(ctx, "Generated reverse zone name", map[string]any{"zone": zoneName})

	zone := ZoneInfo{
		Name:        zoneName,
		Kind:        plan.Kind.ValueString(),
		Nameservers: stringsFromList(ctx, plan.Nameservers, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	createdZone, err := r.clients.PDNS.CreateZone(ctx, zone)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create reverse zone", err.Error())
		return
	}

	plan.ID = types.StringValue(createdZone.Name)
	tflog.Info(ctx, "Created reverse zone", map[string]any{"id": createdZone.Name})

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Reverse zone not found after creation", createdZone.Name)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *reverseZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state reverseZoneModel
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

func (r *reverseZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state reverseZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	zoneName := state.ID.ValueString()
	ctx = tflog.SetField(ctx, "zone", zoneName)

	if !plan.Nameservers.Equal(state.Nameservers) {
		tflog.Debug(ctx, "Updating nameservers for reverse zone")

		zone, err := r.clients.PDNS.GetZone(ctx, zoneName)
		if err != nil {
			resp.Diagnostics.AddError("Couldn't fetch zone", err.Error())
			return
		}

		// Update nameservers in zone object
		zone.Nameservers = stringsFromList(ctx, plan.Nameservers, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		// Build update request
		zoneInfo := ZoneInfoUpd{
			Name:       zoneName,
			Kind:       zone.Kind,
			Account:    zone.Account,
			SoaEditAPI: zone.SoaEditAPI,
		}

		if err := r.clients.PDNS.UpdateZone(ctx, zoneName, zoneInfo); err != nil {
			resp.Diagnostics.AddError("Error updating zone", err.Error())
			return
		}

		// Update NS records to reflect nameserver list
		rrSet := ResourceRecordSet{
			Name:       zoneName,
			Type:       "NS",
			TTL:        3600,
			ChangeType: "REPLACE",
			Records:    make([]Record, len(zone.Nameservers)),
		}

		for i, ns := range zone.Nameservers {
			rrSet.Records[i] = Record{
				Content: ns,
				TTL:     3600,
			}
		}

		if _, err := r.clients.PDNS.ReplaceRecordSet(ctx, zoneName, rrSet); err != nil {
			resp.Diagnostics.AddError("Error updating nameserver records", err.Error())
			return
		}

		tflog.Info(ctx, "Updated nameservers for reverse zone")
	}

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Reverse zone not found after update", zoneName)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *reverseZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state reverseZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zoneName := state.ID.ValueString()

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Deleting reverse zone")

	if err := r.clients.PDNS.DeleteZone(ctx, zoneName); err != nil {
		resp.Diagnostics.AddError("Error deleting zone", err.Error())
		return
	}
	tflog.Info(ctx, "Deleted reverse zone")
}
