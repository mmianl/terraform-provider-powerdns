package powerdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
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

// refresh loads the zone into m. known holds the address each host name
// resolved to the last time this provider wrote or read the zone, and
// justWritten says the zone was written a moment ago. It returns whether the
// zone exists and the addresses to remember for the next read.
func (r *recursorForwardZoneResource) refresh(ctx context.Context, m *recursorForwardZoneModel, known map[string]string, justWritten bool, diags *diag.Diagnostics) (bool, map[string]string) {
	client := r.clients.recursorClient(diags)
	if client == nil {
		return false, nil
	}
	zoneName := m.ID.ValueString()

	ctx = tflog.SetField(ctx, "zone", zoneName)
	tflog.Debug(ctx, "Reading recursor forward zone")

	zone, err := client.GetForwardZone(ctx, zoneName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Recursor forward zone not found; removing from state", map[string]any{"zone": zoneName})
			return false, nil
		}
		diags.AddError("Failed to read recursor forward zone", fmt.Sprintf("failed to read recursor forward zone %q: %s", zoneName, err))
		return false, nil
	}

	// Updates recreate a Forwarded zone, so accepting any other kind would
	// silently change its DNS behavior on the next servers update.
	if zone.Kind != "Forwarded" {
		diags.AddError("Unsupported recursor zone kind", fmt.Sprintf(
			"recursor zone %q has kind %q on the server; this resource only manages Forwarded zones", zoneName, zone.Kind))
		return false, nil
	}

	// This resource only ever creates zones with recursion disabled, and the
	// schema has no field to preserve a server's existing "true" through
	// state. Importing one and reporting no diff would let a later update
	// silently delete and recreate it with recursion turned off.
	if zone.RecursionDesired {
		diags.AddError("Unsupported recursor forward zone", fmt.Sprintf(
			"recursor forward zone %q has recursion_desired=true on the server, which this resource cannot represent; "+
				"it always manages zones with recursion disabled", zoneName))
		return false, nil
	}

	// The Recursor API appends ":53" to servers that were created without
	// an explicit port.  Strip the default suffix so the state matches
	// what the user originally configured.
	servers := make([]string, len(zone.Servers))
	for i, s := range zone.Servers {
		servers[i] = strings.TrimSuffix(s, ":53")
	}
	// The Recursor also resolves a host name to its address before it stores
	// the zone. Keep the host name from the plan or state while the server
	// still reports the address it had for it, otherwise every run would see a
	// change. A different address is real drift and is shown.
	var resolved map[string]string
	if !m.Servers.IsNull() && !m.Servers.IsUnknown() {
		var held diag.Diagnostics
		current := stringsFromList(ctx, m.Servers, &held)
		if !held.HasError() {
			servers, resolved = reconcileServers(current, servers, known, justWritten)
		}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, servers)
	diags.Append(d...)
	if d.HasError() {
		return false, nil
	}
	m.Zone = types.StringValue(zone.Name)
	m.Kind = types.StringValue(zone.Kind)
	m.Servers = list
	return true, resolved
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
	found, resolved := r.refresh(ctx, &plan, nil, true, &resp.Diagnostics)
	if !found {
		if resp.Diagnostics.HasError() {
			keepCreated(ctx, req.Plan, &resp.State, zoneName, &resp.Diagnostics)
		} else {
			resp.Diagnostics.AddError("Recursor forward zone not found after creation", zoneName)
		}
		return
	}
	resp.Diagnostics.Append(saveResolvedServers(ctx, resp.Private, resolved)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *recursorForwardZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recursorForwardZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	known, d := loadResolvedServers(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	found, resolved := r.refresh(ctx, &state, known, false, &resp.Diagnostics)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(saveResolvedServers(ctx, resp.Private, resolved)...)
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

	found, resolved := r.refresh(ctx, &plan, nil, true, &resp.Diagnostics)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Recursor forward zone not found after update", zoneName)
		}
		return
	}
	resp.Diagnostics.Append(saveResolvedServers(ctx, resp.Private, resolved)...)
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

// splitServer splits a server entry into its host and port. An entry without
// a port uses the default DNS port.
func splitServer(entry string) (string, string) {
	host, port, err := net.SplitHostPort(entry)
	if err != nil {
		return strings.Trim(entry, "[]"), "53"
	}
	return host, port
}

// sameServerAddress reports whether two server entries name the same address:
// the same text, or two spellings of one IP address, on the same port. A host
// name only matches itself, because the provider cannot resolve it.
func sameServerAddress(configured, api string) bool {
	if configured == api {
		return true
	}
	configuredHost, configuredPort := splitServer(configured)
	apiHost, apiPort := splitServer(api)
	if configuredPort != apiPort {
		return false
	}
	if strings.EqualFold(configuredHost, apiHost) {
		return true
	}
	configuredIP, apiIP := net.ParseIP(configuredHost), net.ParseIP(apiHost)
	return configuredIP != nil && apiIP != nil && configuredIP.Equal(apiIP)
}

// resolvedTo reports whether api could be the address the Recursor stored for
// the host name in configured: a name that was replaced by an IP address on the
// same port.
func resolvedTo(configured, api string) bool {
	configuredHost, configuredPort := splitServer(configured)
	apiHost, apiPort := splitServer(api)
	return configuredPort == apiPort && net.ParseIP(configuredHost) == nil && net.ParseIP(apiHost) != nil
}

// reconcileServers decides which spelling to keep for each server. An entry
// that names the same address is kept as configured. A host name that the
// Recursor replaced with an address is kept only while that address is the one
// seen before (known) or the zone was just written (justWritten) or nothing is
// known yet. Otherwise the address from the server is shown as a change. The
// second result holds the addresses to remember for host names that were kept.
func reconcileServers(configured, api []string, known map[string]string, justWritten bool) ([]string, map[string]string) {
	if len(configured) != len(api) {
		return api, nil
	}
	servers := make([]string, len(api))
	resolved := map[string]string{}
	for i, address := range api {
		servers[i] = address
		switch {
		case sameServerAddress(configured[i], address):
			servers[i] = configured[i]
		case resolvedTo(configured[i], address):
			previous, seen := known[configured[i]]
			if justWritten || !seen || previous == address {
				servers[i] = configured[i]
				resolved[configured[i]] = address
			}
		}
	}
	return servers, resolved
}

const resolvedServersKey = "resolved_servers"

// privateStore is the private state of a resource, which is not shown in plans.
type privateStore interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// hasPrivateState reports whether the store can be used. A request made
// without private state, as in a unit test, has none.
func hasPrivateState(store privateStore) bool {
	if store == nil {
		return false
	}
	v := reflect.ValueOf(store)
	return v.Kind() != reflect.Pointer || !v.IsNil()
}

// loadResolvedServers reads the addresses remembered for host names.
func loadResolvedServers(ctx context.Context, store privateStore) (map[string]string, diag.Diagnostics) {
	if !hasPrivateState(store) {
		return nil, nil
	}
	raw, diags := store.GetKey(ctx, resolvedServersKey)
	if diags.HasError() || len(raw) == 0 {
		return nil, diags
	}
	known := map[string]string{}
	if err := json.Unmarshal(raw, &known); err != nil {
		// Unreadable data is the same as nothing remembered.
		return nil, nil
	}
	return known, diags
}

// saveResolvedServers remembers the addresses of the host names that were kept.
func saveResolvedServers(ctx context.Context, store privateStore, resolved map[string]string) diag.Diagnostics {
	if !hasPrivateState(store) {
		return nil
	}
	raw, err := json.Marshal(resolved)
	if err != nil {
		return nil
	}
	return store.SetKey(ctx, resolvedServersKey, raw)
}
