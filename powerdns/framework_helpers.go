package powerdns

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// clientsFromProviderData extracts the shared clients handed over by the
// provider. It is nil before the provider is configured.
func clientsFromProviderData(data any, diags *diag.Diagnostics) *ProviderClients {
	if data == nil {
		return nil
	}
	clients, ok := data.(*ProviderClients)
	if !ok {
		diags.AddError("Unexpected provider data", "Expected *ProviderClients; please report this bug.")
		return nil
	}
	return clients
}

// recursorClient returns the recursor client or records an error when
// recursor_server_url was not configured.
func (c *ProviderClients) recursorClient(diags *diag.Diagnostics) *RecursorClient {
	if c == nil || c.Recursor == nil {
		diags.AddError(
			"Recursor client not configured",
			"The 'recursor_server_url' provider argument must be set to manage recursor forward zones.",
		)
		return nil
	}
	return c.Recursor
}

// stringOrNull maps "" to null, for optional attributes the API reports as an
// empty string when unset.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func stringsFromList(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	out := []string{}
	if l.IsNull() || l.IsUnknown() {
		return out
	}
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

func stringsFromSet(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	out := []string{}
	if s.IsNull() || s.IsUnknown() {
		return out
	}
	diags.Append(s.ElementsAs(ctx, &out, false)...)
	sort.Strings(out)
	return out
}

// keepCreated saves what is known about a resource that the server has already
// created, for the case where reading it back fails. Without it Terraform has
// no record of the resource, and the next apply tries to create it again.
// Terraform marks the saved resource as tainted, so the next apply replaces it.
func keepCreated(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, id string, diags *diag.Diagnostics) {
	raw, err := tftypes.Transform(plan.Raw, func(_ *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !v.IsKnown() {
			return tftypes.NewValue(v.Type(), nil), nil
		}
		return v, nil
	})
	if err != nil {
		diags.AddWarning("Could not save the created resource", err.Error())
		return
	}
	state.Raw = raw
	diags.Append(state.SetAttribute(ctx, path.Root("id"), id)...)
}
