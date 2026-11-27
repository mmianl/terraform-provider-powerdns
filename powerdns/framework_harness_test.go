package powerdns

import (
	"context"
	"math/big"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The helpers below call framework resources and data sources directly with
// hand-built plan, config and state values. They cover what the provider does
// with a request (payloads, refresh, error handling) without a Terraform
// binary; plan behavior across real runs is covered by the acceptance tests.

func tfString(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }
func tfBool(b bool) tftypes.Value     { return tftypes.NewValue(tftypes.Bool, b) }
func tfInt(i int64) tftypes.Value     { return tftypes.NewValue(tftypes.Number, i) }

func tfStrings(kind tftypes.Type, values ...string) tftypes.Value {
	items := make([]tftypes.Value, len(values))
	for i, v := range values {
		items[i] = tfString(v)
	}
	return tftypes.NewValue(kind, items)
}

func tfStringList(values ...string) tftypes.Value {
	return tfStrings(tftypes.List{ElementType: tftypes.String}, values...)
}

func tfStringSet(values ...string) tftypes.Value {
	return tfStrings(tftypes.Set{ElementType: tftypes.String}, values...)
}

// tfAttr returns one attribute of an object value.
func tfAttr(t *testing.T, object tftypes.Value, name string) tftypes.Value {
	t.Helper()
	attrs := map[string]tftypes.Value{}
	if err := object.As(&attrs); err != nil {
		t.Fatalf("reading object attributes: %v", err)
	}
	return attrs[name]
}

// tfAttrString returns a string attribute, or "" when it is null.
func tfAttrString(t *testing.T, object tftypes.Value, name string) string {
	t.Helper()
	v := tfAttr(t, object, name)
	if v.IsNull() || !v.IsKnown() {
		return ""
	}
	var s string
	if err := v.As(&s); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return s
}

func tfAttrStrings(t *testing.T, object tftypes.Value, name string) []string {
	t.Helper()
	v := tfAttr(t, object, name)
	if v.IsNull() || !v.IsKnown() {
		return nil
	}
	var items []tftypes.Value
	if err := v.As(&items); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		var s string
		if err := item.As(&s); err != nil {
			t.Fatalf("reading %s element: %v", name, err)
		}
		out = append(out, s)
	}
	return out
}

func tfAttrBool(t *testing.T, object tftypes.Value, name string) (value bool, set bool) {
	t.Helper()
	v := tfAttr(t, object, name)
	if v.IsNull() || !v.IsKnown() {
		return false, false
	}
	if err := v.As(&value); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return value, true
}

func tfAttrInt(t *testing.T, object tftypes.Value, name string) int64 {
	t.Helper()
	v := tfAttr(t, object, name)
	if v.IsNull() || !v.IsKnown() {
		return 0
	}
	n := new(big.Float)
	if err := v.As(&n); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	i, _ := n.Int64()
	return i
}

type resourceHarness struct {
	t      *testing.T
	res    resource.Resource
	schema schema.Schema
	typ    tftypes.Object
}

func newResourceHarness(t *testing.T, res resource.Resource, clients *ProviderClients) *resourceHarness {
	t.Helper()
	ctx := context.Background()
	if c, ok := res.(resource.ResourceWithConfigure); ok {
		var resp resource.ConfigureResponse
		c.Configure(ctx, resource.ConfigureRequest{ProviderData: clients}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("configuring resource: %v", resp.Diagnostics)
		}
	}
	var sresp resource.SchemaResponse
	res.Schema(ctx, resource.SchemaRequest{}, &sresp)
	typ, ok := sresp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("resource schema is not an object")
	}
	return &resourceHarness{t: t, res: res, schema: sresp.Schema, typ: typ}
}

// object builds a full object value; attributes that are not given are null.
func (h *resourceHarness) object(values map[string]tftypes.Value) tftypes.Value {
	return objectOf(h.typ, values)
}

func objectOf(typ tftypes.Object, values map[string]tftypes.Value) tftypes.Value {
	full := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, attrType := range typ.AttributeTypes {
		if v, ok := values[name]; ok {
			full[name] = v
			continue
		}
		full[name] = tftypes.NewValue(attrType, nil)
	}
	return tftypes.NewValue(typ, full)
}

// plan derives a plan from a config the way the framework does for attributes
// without a configured value: computed ones become unknown on create and keep
// the prior value otherwise.
func (h *resourceHarness) plan(config tftypes.Value, prior *tftypes.Value) tftypes.Value {
	configAttrs := map[string]tftypes.Value{}
	if err := config.As(&configAttrs); err != nil {
		h.t.Fatalf("reading config: %v", err)
	}
	// Work on a copy: As may hand back the map the config value holds, and the
	// config must stay as written.
	attrs := make(map[string]tftypes.Value, len(configAttrs))
	for name, v := range configAttrs {
		attrs[name] = v
	}
	var priorAttrs map[string]tftypes.Value
	if prior != nil && !prior.IsNull() {
		priorAttrs = map[string]tftypes.Value{}
		if err := prior.As(&priorAttrs); err != nil {
			h.t.Fatalf("reading prior state: %v", err)
		}
	}
	for name, attr := range h.schema.Attributes {
		if !attr.IsComputed() {
			continue
		}
		if v := attrs[name]; !v.IsNull() {
			continue
		}
		if prev, ok := priorAttrs[name]; ok {
			attrs[name] = prev
			continue
		}
		attrs[name] = tftypes.NewValue(h.typ.AttributeTypes[name], tftypes.UnknownValue)
	}
	return tftypes.NewValue(h.typ, attrs)
}

func (h *resourceHarness) nullState() tfsdk.State {
	return tfsdk.State{Schema: h.schema, Raw: tftypes.NewValue(h.typ, nil)}
}

// create plans and creates from a config, returning the new state.
func (h *resourceHarness) create(config map[string]tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	cfg := h.object(config)
	plan := h.plan(cfg, nil)
	resp := resource.CreateResponse{State: h.nullState()}
	h.res.Create(context.Background(), resource.CreateRequest{
		Config: tfsdk.Config{Schema: h.schema, Raw: cfg},
		Plan:   tfsdk.Plan{Schema: h.schema, Raw: plan},
	}, &resp)
	return resp.State.Raw, resp.Diagnostics
}

// update plans and applies a new config on top of a prior state.
func (h *resourceHarness) update(prior tftypes.Value, config map[string]tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	return h.updateWithUnknown(prior, config)
}

// updateWithUnknown is update, with the named computed attributes planned as
// unknown, as Terraform does for a computed value the server may change.
func (h *resourceHarness) updateWithUnknown(prior tftypes.Value, config map[string]tftypes.Value, unknown ...string) (tftypes.Value, diag.Diagnostics) {
	cfg := h.object(config)
	plan := h.plan(cfg, &prior)
	if len(unknown) > 0 {
		planAttrs := map[string]tftypes.Value{}
		if err := plan.As(&planAttrs); err != nil {
			h.t.Fatalf("reading plan: %v", err)
		}
		attrs := make(map[string]tftypes.Value, len(planAttrs))
		for name, v := range planAttrs {
			attrs[name] = v
		}
		for _, name := range unknown {
			attrs[name] = tftypes.NewValue(h.typ.AttributeTypes[name], tftypes.UnknownValue)
		}
		plan = tftypes.NewValue(h.typ, attrs)
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: h.schema, Raw: prior}}
	h.res.Update(context.Background(), resource.UpdateRequest{
		Config: tfsdk.Config{Schema: h.schema, Raw: cfg},
		Plan:   tfsdk.Plan{Schema: h.schema, Raw: plan},
		State:  tfsdk.State{Schema: h.schema, Raw: prior},
	}, &resp)
	return resp.State.Raw, resp.Diagnostics
}

// read refreshes a state, returning the new state (null when removed).
func (h *resourceHarness) read(prior tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	resp := resource.ReadResponse{State: tfsdk.State{Schema: h.schema, Raw: prior}}
	h.res.Read(context.Background(), resource.ReadRequest{
		State: tfsdk.State{Schema: h.schema, Raw: prior},
	}, &resp)
	return resp.State.Raw, resp.Diagnostics
}

// importState runs ImportState for an id and then Read, as Terraform does.
func (h *resourceHarness) importState(id string) (tftypes.Value, diag.Diagnostics) {
	imp, ok := h.res.(resource.ResourceWithImportState)
	if !ok {
		h.t.Fatalf("resource does not support import")
	}
	resp := resource.ImportStateResponse{State: h.nullState()}
	imp.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
	if resp.Diagnostics.HasError() {
		return resp.State.Raw, resp.Diagnostics
	}
	state, diags := h.read(resp.State.Raw)
	resp.Diagnostics.Append(diags...)
	return state, resp.Diagnostics
}

func readDataSource(t *testing.T, ds datasource.DataSource, clients *ProviderClients, config map[string]tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	t.Helper()
	ctx := context.Background()
	if c, ok := ds.(datasource.DataSourceWithConfigure); ok {
		var resp datasource.ConfigureResponse
		c.Configure(ctx, datasource.ConfigureRequest{ProviderData: clients}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("configuring data source: %v", resp.Diagnostics)
		}
	}
	var sresp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &sresp)
	typ, ok := sresp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("data source schema is not an object")
	}
	cfg := objectOf(typ, config)
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, nil)}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sresp.Schema, Raw: cfg}}, &resp)
	return resp.State.Raw, resp.Diagnostics
}

// destroy runs Delete for a prior state.
func (h *resourceHarness) destroy(prior tftypes.Value) diag.Diagnostics {
	resp := resource.DeleteResponse{State: tfsdk.State{Schema: h.schema, Raw: prior}}
	h.res.Delete(context.Background(), resource.DeleteRequest{
		State: tfsdk.State{Schema: h.schema, Raw: prior},
	}, &resp)
	return resp.Diagnostics
}

// writesSucceedReadsFail answers every write with the given status and body and
// fails every read, except that the first okReads reads return okBody. Some
// creates look the record up before they write it.
func writesSucceedReadsFail(writeStatus int, writeBody string, okReads int, okBody string) roundTripFunc {
	reads := 0
	return func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			return jsonResponse(writeStatus, writeBody), nil
		}
		reads++
		if reads <= okReads {
			return jsonResponse(http.StatusOK, okBody), nil
		}
		return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
	}
}

// assertCreateKeepsState checks that a create which the server accepted, but
// which cannot be read back, still reports the error and leaves the resource in
// state with its id. Otherwise the next apply would create it a second time.
func assertCreateKeepsState(t *testing.T, res resource.Resource, clients *ProviderClients, config map[string]tftypes.Value, wantID string) {
	t.Helper()
	h := newResourceHarness(t, res, clients)
	state, diags := h.create(config)
	if !diags.HasError() {
		t.Fatalf("the failed read-back must be reported")
	}
	if state.IsNull() {
		t.Fatalf("the created resource must be saved")
	}
	if got := tfAttrString(t, state, "id"); got != wantID {
		t.Fatalf("saved id: got %q, want %q", got, wantID)
	}
}
