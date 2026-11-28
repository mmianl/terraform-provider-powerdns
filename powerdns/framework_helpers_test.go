package powerdns

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
)

func TestKeepCreatedSavesKnownValuesAndTheID(t *testing.T) {
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id": tftypes.String, "name": tftypes.String, "serial": tftypes.Number,
	}}
	plan := tftypes.NewValue(typ, map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":   tftypes.NewValue(tftypes.String, "kept"),
		"serial": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	})
	sch := schema.Schema{Attributes: map[string]schema.Attribute{
		"id":     schema.StringAttribute{Computed: true},
		"name":   schema.StringAttribute{Required: true},
		"serial": schema.Int64Attribute{Computed: true},
	}}
	state := tfsdk.State{Schema: sch, Raw: tftypes.NewValue(typ, nil)}
	var diags diag.Diagnostics

	keepCreated(context.Background(), tfsdk.Plan{Schema: sch, Raw: plan}, &state, "kept-id", &diags)

	assert.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, "kept-id", tfAttrString(t, state.Raw, "id"))
	assert.Equal(t, "kept", tfAttrString(t, state.Raw, "name"))
	assert.True(t, tfAttr(t, state.Raw, "serial").IsNull(), "an unknown value must not be saved")
}

// A plan that cannot be read leaves the state alone and says so, instead of
// failing the apply a second time.
func TestKeepCreatedWarnsWhenThePlanCannotBeSaved(t *testing.T) {
	state := tfsdk.State{}
	var diags diag.Diagnostics

	keepCreated(context.Background(), tfsdk.Plan{Raw: tftypes.Value{}}, &state, "kept-id", &diags)

	assert.False(t, diags.HasError(), "a warning, not an error")
	assert.Equal(t, 1, diags.WarningsCount())
	assert.Equal(t, "Could not save the created resource", diags[0].Summary())
	assert.True(t, state.Raw.Equal(tftypes.Value{}), "the state must stay as it was")
}
