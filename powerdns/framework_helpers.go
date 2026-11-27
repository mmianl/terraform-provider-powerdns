package powerdns

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
