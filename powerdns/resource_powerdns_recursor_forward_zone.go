package powerdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

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
	_ resource.Resource                = &recursorForwardZoneResource{}
	_ resource.ResourceWithConfigure   = &recursorForwardZoneResource{}
	_ resource.ResourceWithImportState = &recursorForwardZoneResource{}
)

func newRecursorForwardZoneResource() resource.Resource { return &recursorForwardZoneResource{} }

type recursorForwardZoneResource struct {
	clients *ProviderClients
}

type recursorForwardZoneModel struct {
	ID      types.String `tfsdk:"id"`
	Zone    types.String `tfsdk:"zone"`
	Kind    types.String `tfsdk:"kind"`
	Servers types.List   `tfsdk:"servers"`
}

func (r *recursorForwardZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recursor_forward_zone"
}

func (r *recursorForwardZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PowerDNS Recursor forward zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The forward zone name, with a trailing dot.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateFQDN)},
				Description:   "The zone name to forward. Must be a fully qualified domain name ending with a trailing dot.",
			},
			"kind": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "The zone kind reported by the Recursor API. This resource only manages Forwarded zones.",
			},
			"servers": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "List of DNS servers to forward queries to",
			},
		},
	}
}

func (r *recursorForwardZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// ImportState works from the zone name alone, which is the ID.
func (r *recursorForwardZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *recursorForwardZoneResource) refresh(ctx context.Context, m *recursorForwardZoneModel, diags *diag.Diagnostics) bool {
	client := r.clients.recursorClient(diags)
	if client == nil {
		return false
	}
	zoneName := m.ID.ValueString()

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Reading recursor forward zone")

	zone, err := client.GetForwardZone(ctx, zoneName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor forward zone not found; removing from state", map[string]any{"zone": zoneName})
			return false
		}
		diags.AddError("Failed to read recursor forward zone", fmt.Sprintf("failed to read recursor forward zone %q: %s", zoneName, err))
		return false
	}

	// Updates recreate a Forwarded zone, so accepting any other kind would
	// silently change its DNS behavior on the next servers update.
	if zone.Kind != "Forwarded" {
		diags.AddError("Unsupported recursor zone kind", fmt.Sprintf(
			"recursor zone %q has kind %q on the server; this resource only manages Forwarded zones", zoneName, zone.Kind))
		return false
	}

	// This resource only ever creates zones with recursion disabled, and the
	// schema has no field to preserve a server's existing "true" through
	// state. Importing one and reporting no diff would let a later update
	// silently delete and recreate it with recursion turned off.
	if zone.RecursionDesired {
		diags.AddError("Unsupported recursor forward zone", fmt.Sprintf(
			"recursor forward zone %q has recursion_desired=true on the server, which this resource cannot represent; "+
				"it always manages zones with recursion disabled", zoneName))
		return false
	}

	// The Recursor API appends ":53" to servers that were created without
	// an explicit port.  Strip the default suffix so the state matches
	// what the user originally configured.
	servers := make([]string, len(zone.Servers))
	for i, s := range zone.Servers {
		servers[i] = strings.TrimSuffix(s, ":53")
	}
	// The Recursor also resolves a host name to its address before it stores
	// the zone. Keep the host name from the plan or state when the server
	// reports an address for it, otherwise every run would see a change.
	if !m.Servers.IsNull() && !m.Servers.IsUnknown() {
		var held diag.Diagnostics
		current := stringsFromList(ctx, m.Servers, &held)
		if !held.HasError() && len(current) == len(servers) {
			for i := range servers {
				if sameServerAddress(current[i], servers[i]) {
					servers[i] = current[i]
				}
			}
		}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, servers)
	diags.Append(d...)
	if d.HasError() {
		return false
	}
	m.Zone = types.StringValue(zone.Name)
	m.Kind = types.StringValue(zone.Kind)
	m.Servers = list
	return true
}

func (r *recursorForwardZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan recursorForwardZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.clients.recursorClient(&resp.Diagnostics)
	if client == nil {
		return
	}

	zoneName := plan.Zone.ValueString()
	servers := stringsFromList(ctx, plan.Servers, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Creating recursor forward zone")

	zone := &RecursorForwardZone{
		Name:             zoneName,
		Type:             "Zone",
		Kind:             "Forwarded",
		Servers:          servers,
		RecursionDesired: false,
	}
	if err := client.CreateForwardZone(ctx, zone); err != nil {
		resp.Diagnostics.AddError("Failed to create recursor forward zone", fmt.Sprintf("failed to create recursor forward zone %q: %s", zoneName, err))
		return
	}

	plan.ID = types.StringValue(zoneName)
	tflog.Info(ctx, "Created recursor forward zone", map[string]any{"id": zoneName})
	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if resp.Diagnostics.HasError() {
			keepCreated(ctx, req.Plan, &resp.State, zoneName, &resp.Diagnostics)
		} else {
			resp.Diagnostics.AddError("Recursor forward zone not found after creation", zoneName)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorForwardZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recursorForwardZoneModel
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

func (r *recursorForwardZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state recursorForwardZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.clients.recursorClient(&resp.Diagnostics)
	if client == nil {
		return
	}

	plan.ID = state.ID
	zoneName := state.ID.ValueString()
	servers := stringsFromList(ctx, plan.Servers, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Updating recursor forward zone")

	// update as delete then create
	if err := client.DeleteForwardZone(ctx, zoneName); err != nil && !errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("Failed to update recursor forward zone", fmt.Sprintf("failed to delete existing recursor forward zone %q before update: %s", zoneName, err))
		return
	}

	zone := &RecursorForwardZone{
		Name:             zoneName,
		Type:             "Zone",
		Kind:             "Forwarded",
		Servers:          servers,
		RecursionDesired: false,
	}
	if err := client.CreateForwardZone(ctx, zone); err != nil {
		resp.Diagnostics.AddError("Failed to update recursor forward zone", fmt.Sprintf("failed to recreate recursor forward zone %q during update: %s", zoneName, err))
		return
	}

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Recursor forward zone not found after update", zoneName)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorForwardZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state recursorForwardZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.clients.recursorClient(&resp.Diagnostics)
	if client == nil {
		return
	}
	zoneName := state.ID.ValueString()

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Deleting recursor forward zone")

	if err := client.DeleteForwardZone(ctx, zoneName); err != nil && !errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("Error deleting recursor forward zone", fmt.Sprintf("error deleting recursor forward zone %q: %s", zoneName, err))
		return
	}
	tflog.Info(ctx, "Successfully deleted recursor forward zone", map[string]any{"zone": zoneName})
}

// sameServerAddress reports whether the server entry written in the
// configuration (a host name or an address, with an optional port) is what the
// Recursor reports as api. The provider cannot resolve names, so a host name
// matches any address on the same port.
func sameServerAddress(configured, api string) bool {
	if configured == api {
		return true
	}
	split := func(entry string) (string, string) {
		host, port, err := net.SplitHostPort(entry)
		if err != nil {
			return strings.Trim(entry, "[]"), "53"
		}
		return host, port
	}
	configuredHost, configuredPort := split(configured)
	apiHost, apiPort := split(api)
	if configuredPort != apiPort {
		return false
	}
	if strings.EqualFold(configuredHost, apiHost) {
		return true
	}
	// Two spellings of the same IP address, such as an expanded and a
	// compressed IPv6 address.
	configuredIP, apiIP := net.ParseIP(configuredHost), net.ParseIP(apiHost)
	if configuredIP != nil && apiIP != nil {
		return configuredIP.Equal(apiIP)
	}
	// A host name the Recursor replaced with its address.
	return net.ParseIP(configuredHost) == nil && net.ParseIP(apiHost) != nil
}
