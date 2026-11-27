package powerdns

import (
	"context"
	"errors"
	"fmt"

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

// writableRecursorSettings lists the settings the recursor API accepts a write
// for. Verified against Recursor 5.4.4: ws-recursor.cc registers handlers for
// /config/allow-from and /config/allow-notify-from only, and any other name
// returns 404 for both GET and PUT.
var writableRecursorSettings = []string{
	"allow-from",
	"allow-notify-from",
}

var (
	_ resource.Resource                = &recursorConfigResource{}
	_ resource.ResourceWithConfigure   = &recursorConfigResource{}
	_ resource.ResourceWithImportState = &recursorConfigResource{}
)

func newRecursorConfigResource() resource.Resource { return &recursorConfigResource{} }

type recursorConfigResource struct {
	clients *ProviderClients
}

type recursorConfigModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Value types.Set    `tfsdk:"value"`
}

func (r *recursorConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recursor_config"
}

func (r *recursorConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a writable PowerDNS Recursor setting.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The name of the configuration setting.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				// The recursor exposes exactly two settings for writing. Every
				// other name answers 404 on both GET and PUT, so accepting one
				// only defers the failure to apply.
				Validators: []validator.String{stringvalidator.OneOf(writableRecursorSettings...)},
				Description: "The name of the recursor config setting. The recursor API only " +
					"exposes allow-from and allow-notify-from for writing.",
			},
			"value": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "The value of the recursor config setting (list of CIDRs, etc.)",
			},
		},
	}
}

func (r *recursorConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// ImportState works from the setting name alone, which is the ID.
func (r *recursorConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *recursorConfigResource) write(ctx context.Context, m *recursorConfigModel, diags *diag.Diagnostics) {
	client := r.clients.recursorClient(diags)
	if client == nil {
		return
	}
	name := m.Name.ValueString()
	values := stringsFromSet(ctx, m.Value, diags)
	if diags.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Writing recursor config")

	if err := client.SetConfig(ctx, name, values); err != nil {
		diags.AddError("Failed to write recursor config", fmt.Sprintf("failed to write recursor config %q: %s", name, err))
		return
	}

	m.ID = types.StringValue(name)
	if !r.refresh(ctx, m, diags) && !diags.HasError() {
		diags.AddError("Recursor config not found after write", name)
	}
}

func (r *recursorConfigResource) refresh(ctx context.Context, m *recursorConfigModel, diags *diag.Diagnostics) bool {
	client := r.clients.recursorClient(diags)
	if client == nil {
		return false
	}

	name := m.ID.ValueString()
	ctx = tflog.SetField(ctx, "recursor_config_name", name)
	tflog.Debug(ctx, "Reading recursor config")

	setting, err := client.GetConfig(ctx, name)
	if err != nil {
		// Only treat "not found" as removing from state, other errors should fail
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor config not found; removing from state")
			return false
		}
		diags.AddError("Failed to get recursor config", fmt.Sprintf("failed to get recursor config %q: %s", name, err))
		return false
	}

	set, d := types.SetValueFrom(ctx, types.StringType, setting.Value)
	diags.Append(d...)
	if d.HasError() {
		return false
	}
	m.Name = types.StringValue(setting.Name)
	m.Value = set
	return true
}

func (r *recursorConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan recursorConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "Created recursor config", map[string]any{"id": plan.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recursorConfigModel
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

func (r *recursorConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state recursorConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state recursorConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SetField(ctx, "recursor_config_name", state.ID.ValueString())
	// The API only supports GET and PUT for config, so delete will do nothing
	tflog.Info(ctx, "Successfully deleted recursor config (removed from state)", map[string]any{"name": state.ID.ValueString()})
}
