package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	_ resource.Resource                = &zoneMetadataResource{}
	_ resource.ResourceWithConfigure   = &zoneMetadataResource{}
	_ resource.ResourceWithImportState = &zoneMetadataResource{}
)

func newZoneMetadataResource() resource.Resource { return &zoneMetadataResource{} }

type zoneMetadataResource struct {
	clients *ProviderClients
}

type zoneMetadataModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Kind     types.String `tfsdk:"kind"`
	Metadata types.Set    `tfsdk:"metadata"`
}

func (r *zoneMetadataResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_metadata"
}

func (r *zoneMetadataResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages one kind of PowerDNS zone metadata.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The zone and metadata kind joined by `:::`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateZoneName)},
				Description:   "Zone name, for example \"example.com.\".",
			},
			"kind": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Metadata kind as used by PowerDNS, for example \"ALSO-NOTIFY\" or \"ALLOW-AXFR-FROM\".",
			},
			"metadata": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
				Description: "Set of values for this metadata kind.",
			},
		},
	}
}

func (r *zoneMetadataResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func zoneMetadataID(zone string, kind string) string {
	return zone + idSeparator + kind
}

func parseZoneMetadataID(id string) (string, string, error) {
	parts := strings.Split(id, idSeparator)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid zone metadata id %q, expected <zone>%s<kind>", id, idSeparator)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid zone metadata id %q, zone and kind must be non-empty", id)
	}
	return parts[0], parts[1], nil
}

func (r *zoneMetadataResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	zone, kind, err := parseZoneMetadataID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), zoneMetadataID(zone, kind))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kind"), kind)...)
}

func (r *zoneMetadataResource) write(ctx context.Context, m *zoneMetadataModel, diags *diag.Diagnostics) {
	zone := m.Zone.ValueString()
	kind := m.Kind.ValueString()
	values := stringsFromSet(ctx, m.Metadata, diags)
	if diags.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "kind", kind)
	tflog.Debug(ctx, "Writing PowerDNS zone metadata")

	if err := r.clients.PDNS.ReplaceZoneMetadata(ctx, zone, kind, values); err != nil {
		diags.AddError("Error writing zone metadata", fmt.Sprintf("error writing zone metadata %s for %s: %s", kind, zone, err))
		return
	}

	m.ID = types.StringValue(zoneMetadataID(zone, kind))
	if !r.refresh(ctx, m, diags) && !diags.HasError() {
		diags.AddError("Zone metadata not found after write", m.ID.ValueString())
	}
}

// refresh loads the metadata into m, reporting false if it is gone.
func (r *zoneMetadataResource) refresh(ctx context.Context, m *zoneMetadataModel, diags *diag.Diagnostics) bool {
	zone := m.Zone.ValueString()
	kind := m.Kind.ValueString()

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "kind", kind)
	tflog.Debug(ctx, "Reading PowerDNS zone metadata")

	allMetadata, err := r.clients.PDNS.ListZoneMetadata(ctx, zone)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing metadata from state")
			return false
		}
		diags.AddError("Couldn't fetch zone metadata", fmt.Sprintf("couldn't fetch zone metadata for %s: %s", zone, err))
		return false
	}

	values := getMetadataValues(allMetadata, kind)
	if len(values) == 0 {
		tflog.Warn(ctx, "Zone metadata not found; removing from state")
		return false
	}

	set, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	if d.HasError() {
		return false
	}
	m.Metadata = set
	return true
}

func (r *zoneMetadataResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zoneMetadataModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneMetadataResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zoneMetadataModel
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

func (r *zoneMetadataResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan zoneMetadataModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneMetadataResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zoneMetadataModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := state.Zone.ValueString()
	kind := state.Kind.ValueString()

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "kind", kind)
	tflog.Debug(ctx, "Deleting PowerDNS zone metadata")

	if err := r.clients.PDNS.DeleteZoneMetadata(ctx, zone, kind); err != nil {
		resp.Diagnostics.AddError("Error deleting zone metadata", fmt.Sprintf("error deleting zone metadata %s for %s: %s", kind, zone, err))
	}
}
