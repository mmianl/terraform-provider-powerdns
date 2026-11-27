package powerdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

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
	_ resource.Resource                = &networkResource{}
	_ resource.ResourceWithConfigure   = &networkResource{}
	_ resource.ResourceWithImportState = &networkResource{}
)

func newNetworkResource() resource.Resource { return &networkResource{} }

type networkResource struct {
	clients *ProviderClients
}

type networkModel struct {
	ID      types.String `tfsdk:"id"`
	Network types.String `tfsdk:"network"`
	View    types.String `tfsdk:"view"`
}

func validateCIDRString(v interface{}, k string) ([]string, []error) {
	value, ok := v.(string)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}
	if _, _, err := net.ParseCIDR(value); err != nil {
		return nil, []error{fmt.Errorf("expected %q to contain a valid CIDR, got: %s with err: %s", k, value, err)}
	}
	return nil, nil
}

func (r *networkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *networkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Maps a network (CIDR) to a PowerDNS view.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The network in CIDR notation.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network": schema.StringAttribute{
				Description:   "The network in CIDR notation, such as `192.0.2.0/24`. Changing it forces a new mapping.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(validateCIDRString)},
			},
			"view": schema.StringAttribute{
				Description: "The name of the view the network is mapped to.",
				Required:    true,
				Validators:  []validator.String{validateWith(validateViewName)},
			},
		},
	}
}

func (r *networkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *networkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func parseNetworkCIDR(network string) (string, string, error) {
	ip, ipnet, err := net.ParseCIDR(network)
	if err != nil {
		return "", "", fmt.Errorf("invalid network format: %w", err)
	}
	prefixlen, _ := ipnet.Mask.Size()
	return ip.String(), strconv.Itoa(prefixlen), nil
}

func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkModel
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

func (r *networkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) upsert(ctx context.Context, m *networkModel, diags *diag.Diagnostics) {
	network := m.Network.ValueString()
	view := m.View.ValueString()

	ip, prefixlen, err := parseNetworkCIDR(network)
	if err != nil {
		diags.AddError("Invalid network", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "network", network)
	ctx = tflog.SetField(ctx, "view", view)
	tflog.Debug(ctx, "Creating or updating PowerDNS network")

	if err := r.clients.PDNS.SetNetwork(ctx, ip, prefixlen, view); err != nil {
		diags.AddError("Failed to set network", fmt.Sprintf("failed to set network %s: %s", network, err))
		return
	}

	m.ID = types.StringValue(network)
	if !r.refresh(ctx, m, diags) && !diags.HasError() {
		diags.AddError("Network not found after write", network)
	}
}

// refresh loads the network into m, reporting false if it no longer exists.
func (r *networkResource) refresh(ctx context.Context, m *networkModel, diags *diag.Diagnostics) bool {
	networkCIDR := m.ID.ValueString()

	ip, prefixlen, err := parseNetworkCIDR(networkCIDR)
	if err != nil {
		diags.AddError("Invalid network id", err.Error())
		return false
	}

	ctx = tflog.SetField(ctx, "network", networkCIDR)
	tflog.Debug(ctx, "Reading PowerDNS network")

	network, err := r.clients.PDNS.GetNetwork(ctx, ip, prefixlen)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false
		}
		diags.AddError("Failed to get network", fmt.Sprintf("failed to get network %s: %s", networkCIDR, err))
		return false
	}

	m.Network = types.StringValue(network.Network)
	m.View = types.StringValue(network.View)
	return true
}

func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkModel
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

func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	networkCIDR := state.ID.ValueString()

	ip, prefixlen, err := parseNetworkCIDR(networkCIDR)
	if err != nil {
		resp.Diagnostics.AddError("Invalid network id", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "network", networkCIDR)
	tflog.Debug(ctx, "Deleting PowerDNS network")

	if err := r.clients.PDNS.DeleteNetwork(ctx, ip, prefixlen); err != nil && !errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("Failed to delete network", fmt.Sprintf("failed to delete network %s: %s", networkCIDR, err))
	}
}
