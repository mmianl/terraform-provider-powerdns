package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	_ resource.Resource                = &viewZoneAssociationResource{}
	_ resource.ResourceWithConfigure   = &viewZoneAssociationResource{}
	_ resource.ResourceWithImportState = &viewZoneAssociationResource{}
)

func newViewZoneAssociationResource() resource.Resource { return &viewZoneAssociationResource{} }

type viewZoneAssociationResource struct {
	clients *ProviderClients
}

type viewZoneAssociationModel struct {
	ID   types.String `tfsdk:"id"`
	View types.String `tfsdk:"view"`
	Zone types.String `tfsdk:"zone"`
}

func (r *viewZoneAssociationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view_zone_association"
}

func (r *viewZoneAssociationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Associates a zone with a PowerDNS view.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The view and zone names joined by `:::`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"view": schema.StringAttribute{
				Description:   "The name of the view. Changing it forces a new association.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(validateViewName)},
			},
			"zone": schema.StringAttribute{
				Description:   "The zone name, with a trailing dot. Changing it forces a new association.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateZoneName)},
			},
		},
	}
}

func (r *viewZoneAssociationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func viewZoneAssociationID(view string, zone string) string {
	return view + idSeparator + zone
}

func parseViewZoneAssociationID(id string) (string, string, error) {
	parts := strings.Split(id, idSeparator)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid view zone association id %q, expected <view>%s<zone>", id, idSeparator)
	}
	return parts[0], parts[1], nil
}

func (r *viewZoneAssociationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	view, zone, err := parseViewZoneAssociationID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("view"), view)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), zone)...)
}

func (r *viewZoneAssociationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan viewZoneAssociationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	view := plan.View.ValueString()
	zone := plan.Zone.ValueString()

	ctx = tflog.SetField(ctx, "view", view)
	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Creating PowerDNS view zone association")

	if err := r.clients.PDNS.AddZoneToView(ctx, view, zone); err != nil {
		resp.Diagnostics.AddError("Failed to add zone to view", fmt.Sprintf("failed to add zone %s to view %s: %s", zone, view, err))
		return
	}

	plan.ID = types.StringValue(viewZoneAssociationID(view, zone))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *viewZoneAssociationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state viewZoneAssociationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	view, zone, err := parseViewZoneAssociationID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "view", view)
	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Reading PowerDNS view zone association")

	pdnsView, err := r.clients.PDNS.GetView(ctx, view)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to get view", fmt.Sprintf("failed to get view %s: %s", view, err))
		return
	}

	for _, existingZone := range pdnsView.Zones {
		if existingZone == zone {
			state.View = types.StringValue(view)
			state.Zone = types.StringValue(zone)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

// Update is never called: every configurable attribute forces replacement.
func (r *viewZoneAssociationResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "powerdns_view_zone_association cannot be updated in place.")
}

func (r *viewZoneAssociationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state viewZoneAssociationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	view, zone, err := parseViewZoneAssociationID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "view", view)
	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Debug(ctx, "Deleting PowerDNS view zone association")

	if err := r.clients.PDNS.RemoveZoneFromView(ctx, view, zone); err != nil && !errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("Failed to remove zone from view", fmt.Sprintf("failed to remove zone %s from view %s: %s", zone, view, err))
	}
}
