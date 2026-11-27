package powerdns

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Note: the framework calls SemanticEquals on the NEW value and passes the
// PRIOR value as the argument; equal(prior, next) is called accordingly.
//
// semanticStringType is a string whose values may be spelled differently while
// meaning the same thing (case, IPv6 compression). The framework keeps the
// prior value when the new one is semantically equal, which replaces the SDK's
// DiffSuppressFunc and StateFunc.
type semanticStringType struct {
	basetypes.StringType
	name  string
	equal func(prior, next string) bool
}

var (
	_ basetypes.StringTypable                    = semanticStringType{}
	_ basetypes.StringValuableWithSemanticEquals = semanticStringValue{}
)

func (t semanticStringType) Equal(o attr.Type) bool {
	other, ok := o.(semanticStringType)
	return ok && other.name == t.name
}

func (t semanticStringType) String() string { return t.name }

func (t semanticStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return semanticStringValue{StringValue: in, typ: t}, nil
}

func (t semanticStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	v, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	sv, ok := v.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", v)
	}
	return semanticStringValue{StringValue: sv, typ: t}, nil
}

func (t semanticStringType) ValueType(_ context.Context) attr.Value {
	return semanticStringValue{typ: t}
}

type semanticStringValue struct {
	basetypes.StringValue
	typ semanticStringType
}

func (v semanticStringValue) Type(_ context.Context) attr.Type { return v.typ }

func (v semanticStringValue) Equal(o attr.Value) bool {
	other, ok := o.(semanticStringValue)
	return ok && v.StringValue.Equal(other.StringValue)
}

func (v semanticStringValue) StringSemanticEquals(_ context.Context, prior basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	other, ok := prior.(semanticStringValue)
	if !ok {
		diags.AddError("Semantic equality check error", fmt.Sprintf("unexpected value type %T", prior))
		return false, diags
	}
	if v.IsNull() || v.IsUnknown() || other.IsNull() || other.IsUnknown() {
		return v.StringValue.Equal(other.StringValue), diags
	}
	return v.typ.equal(other.ValueString(), v.ValueString()), diags
}

// caseInsensitiveStringType treats "Master" and "master" as the same value.
var caseInsensitiveStringType = semanticStringType{
	name:  "powerdns.CaseInsensitiveStringType",
	equal: strings.EqualFold,
}

// masterAddressType treats equivalent spellings of an IPv4/IPv6 address
// (optionally with a port) as the same value.
var masterAddressType = semanticStringType{
	name: "powerdns.MasterAddressType",
	equal: func(prior, next string) bool {
		return NormalizeMasterAddress(prior) == NormalizeMasterAddress(next)
	},
}

// semanticInt64Type is an Int64 whose prior value can declare the new one
// equivalent.
type semanticInt64Type struct {
	basetypes.Int64Type
	name  string
	equal func(prior, next int64) bool
}

var (
	_ basetypes.Int64Typable                    = semanticInt64Type{}
	_ basetypes.Int64ValuableWithSemanticEquals = semanticInt64Value{}
)

func (t semanticInt64Type) Equal(o attr.Type) bool {
	other, ok := o.(semanticInt64Type)
	return ok && other.name == t.name
}

func (t semanticInt64Type) String() string { return t.name }

func (t semanticInt64Type) ValueFromInt64(_ context.Context, in basetypes.Int64Value) (basetypes.Int64Valuable, diag.Diagnostics) {
	return semanticInt64Value{Int64Value: in, typ: t}, nil
}

func (t semanticInt64Type) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	v, err := t.Int64Type.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	iv, ok := v.(basetypes.Int64Value)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", v)
	}
	return semanticInt64Value{Int64Value: iv, typ: t}, nil
}

func (t semanticInt64Type) ValueType(_ context.Context) attr.Value {
	return semanticInt64Value{typ: t}
}

type semanticInt64Value struct {
	basetypes.Int64Value
	typ semanticInt64Type
}

func (v semanticInt64Value) Type(_ context.Context) attr.Type { return v.typ }

func (v semanticInt64Value) Equal(o attr.Value) bool {
	other, ok := o.(semanticInt64Value)
	return ok && v.Int64Value.Equal(other.Int64Value)
}

func (v semanticInt64Value) Int64SemanticEquals(_ context.Context, prior basetypes.Int64Valuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	other, ok := prior.(semanticInt64Value)
	if !ok {
		diags.AddError("Semantic equality check error", fmt.Sprintf("unexpected value type %T", prior))
		return false, diags
	}
	if v.IsNull() || v.IsUnknown() || other.IsNull() || other.IsUnknown() {
		return v.Int64Value.Equal(other.Int64Value), diags
	}
	return v.typ.equal(other.ValueInt64(), v.ValueInt64()), diags
}

// soaSerialType: a configured serial of 0 means "let soa_edit_api manage it",
// so whatever PowerDNS stores is never drift against that 0.
var soaSerialType = semanticInt64Type{
	name:  "powerdns.SOASerialType",
	equal: func(prior, _ int64) bool { return prior == 0 },
}

// legacyStringValidator adapts the SDK-style validation functions in ip.go.
type legacyStringValidator struct {
	fn func(v interface{}, k string) ([]string, []error)
}

var _ validator.String = legacyStringValidator{}

func validateWith(fn func(v interface{}, k string) ([]string, []error)) validator.String {
	return legacyStringValidator{fn: fn}
}

func (legacyStringValidator) Description(context.Context) string {
	return "value must be valid"
}

func (v legacyStringValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v legacyStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	name := req.Path.String()
	if steps := req.Path.Steps(); len(steps) > 0 {
		name = steps[0].String()
	}
	_, errs := v.fn(req.ConfigValue.ValueString(), name)
	for _, err := range errs {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Attribute Value", err.Error())
	}
}

func newSemanticString(t semanticStringType, s string) semanticStringValue {
	return semanticStringValue{StringValue: basetypes.NewStringValue(s), typ: t}
}

func newSemanticInt64(t semanticInt64Type, i int64) semanticInt64Value {
	return semanticInt64Value{Int64Value: basetypes.NewInt64Value(i), typ: t}
}

// requiresReplaceUnlessEquivalent forces replacement when the configured value
// changes, except when the new value is equivalent to the stored one (for
// example a different spelling of the same name). A null configuration never
// forces replacement; an unknown one always does.
func requiresReplaceUnlessEquivalent(equal func(prior, next string) bool, description string) planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			switch {
			case req.ConfigValue.IsNull():
				resp.RequiresReplace = false
			case req.ConfigValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown():
				resp.RequiresReplace = true
			default:
				resp.RequiresReplace = !equal(req.StateValue.ValueString(), req.PlanValue.ValueString())
			}
		},
		description, description,
	)
}

// keepUnlessRecordsChange keeps the stored value of a computed value that PowerDNS
// derives from the record contents. It stays unknown when the contents change,
// because the server then decides the new value.
type keepUnlessRecordsChange struct{}

var _ planmodifier.Bool = keepUnlessRecordsChange{}

func (keepUnlessRecordsChange) Description(context.Context) string {
	return "Keeps the stored value unless the record contents change."
}

func (m keepUnlessRecordsChange) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepUnlessRecordsChange) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.State.Raw.IsNull() || !req.ConfigValue.IsNull() || !req.PlanValue.IsUnknown() {
		return
	}
	var planned, stored types.Set
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("records"), &planned)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("records"), &stored)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if planned.Equal(stored) {
		resp.PlanValue = req.StateValue
	}
}
