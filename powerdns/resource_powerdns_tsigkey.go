package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
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

// tsigAlgorithms lists the algorithms accepted by the PowerDNS API.
var tsigAlgorithms = []string{
	"hmac-md5",
	"hmac-sha1",
	"hmac-sha224",
	"hmac-sha256",
	"hmac-sha384",
	"hmac-sha512",
}

// tsigNameType treats a TSIG key name with and without its optional trailing
// dot as the same value. PowerDNS returns names without the dot.
var tsigNameType = semanticStringType{
	name: "powerdns.TSIGKeyNameType",
	equal: func(prior, next string) bool {
		return strings.TrimSuffix(prior, ".") == strings.TrimSuffix(next, ".")
	},
}

var (
	_ resource.Resource                = &tsigKeyResource{}
	_ resource.ResourceWithConfigure   = &tsigKeyResource{}
	_ resource.ResourceWithImportState = &tsigKeyResource{}
	_ resource.ResourceWithModifyPlan  = &tsigKeyResource{}
)

func newTSIGKeyResource() resource.Resource { return &tsigKeyResource{} }

type tsigKeyResource struct {
	clients *ProviderClients
}

type tsigKeyModel struct {
	ID           types.String        `tfsdk:"id"`
	Name         semanticStringValue `tfsdk:"name"`
	Algorithm    types.String        `tfsdk:"algorithm"`
	Key          types.String        `tfsdk:"key"`
	KeyWO        types.String        `tfsdk:"key_wo"`
	KeyWOVersion types.Int64         `tfsdk:"key_wo_version"`
}

func (r *tsigKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tsigkey"
}

func (r *tsigKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PowerDNS TSIG key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The TSIG key ID as PowerDNS reports it.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:   true,
				CustomType: tsigNameType,
				// The name is part of the API ID. Replacement makes the new ID
				// unknown during planning so dependent zone references update in
				// the same apply, instead of retaining the removed key's ID.
				PlanModifiers: []planmodifier.String{requiresReplaceUnlessEquivalent(tsigNameType.equal,
					"A different TSIG key name forces a new key; a trailing dot does not.")},
				Description: "The name of the TSIG key. Changing this forces a new key.",
			},
			"algorithm": schema.StringAttribute{
				Required: true,
				// PowerDNS stores a changed algorithm as an extra key under the
				// same name instead of replacing the existing one, so the key has
				// to be recreated rather than updated in place.
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf(tsigAlgorithms...)},
				Description:   "The TSIG algorithm, e.g. hmac-sha256. Changing this forces a new key.",
			},
			"key": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:    []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("key_wo"))},
				Description: "The base64 encoded secret. When omitted, PowerDNS generates " +
					"the key material and it is stored in state.",
			},
			"key_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("key")),
					stringvalidator.AlsoRequires(path.MatchRoot("key_wo_version")),
				},
				Description: "The base64 encoded secret, as a write-only value that is never saved " +
					"to the plan or state. Needs Terraform 1.11 or later. Change key_wo_version to send a new value.",
			},
			"key_wo_version": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("key_wo")),
					int64validator.AtLeast(1),
				},
				Description: "Bump this to make the provider send the current key_wo value to PowerDNS again. " +
					"Terraform cannot see a write-only value change, so this is how a rotation is requested.",
			},
		},
	}
}

func (r *tsigKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// ModifyPlan drops the stored secret from the plan when the key is switched to
// write-only mode, so the planned value matches what is saved afterwards.
func (r *tsigKeyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.Config.Raw.IsNull() {
		return
	}
	var config tsigKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.KeyWOVersion.IsNull() || config.KeyWOVersion.IsUnknown() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("key"), types.StringNull())...)
}

func (r *tsigKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// tsigKeySecret returns the secret to send to PowerDNS. A write-only value
// exists only in the configuration, never in state, so it is read from there.
func tsigKeySecret(config, plan tsigKeyModel) string {
	if plan.KeyWOVersion.ValueInt64() != 0 && !config.KeyWO.IsNull() && !config.KeyWO.IsUnknown() {
		return config.KeyWO.ValueString()
	}
	return plan.Key.ValueString()
}

func (r *tsigKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config tsigKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tsigKey := TSIGKey{
		Name:      plan.Name.ValueString(),
		Algorithm: plan.Algorithm.ValueString(),
		Key:       tsigKeySecret(config, plan),
	}

	ctx = tflog.SetField(ctx, "tsigkey_name", tsigKey.Name)
	tflog.Debug(ctx, "Creating PowerDNS TSIG key")

	created, err := r.clients.PDNS.CreateTSIGKey(ctx, tsigKey)
	if err != nil {
		resp.Diagnostics.AddError("Error creating PowerDNS TSIG key", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	tflog.Info(ctx, "Created PowerDNS TSIG key", map[string]any{"id": created.ID})

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if resp.Diagnostics.HasError() {
			keepCreated(ctx, req.Plan, &resp.State, created.ID, &resp.Diagnostics)
		} else {
			resp.Diagnostics.AddError("TSIG key not found after creation", created.ID)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// refresh loads the key into m, reporting false when it is gone.
func (r *tsigKeyResource) refresh(ctx context.Context, m *tsigKeyModel, diags *diag.Diagnostics) bool {
	id := m.ID.ValueString()
	ctx = tflog.SetField(ctx, "tsigkey_id", id)
	tflog.Debug(ctx, "Reading PowerDNS TSIG key")

	tsigKey, err := r.clients.PDNS.GetTSIGKey(ctx, id, m.Algorithm.ValueString())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "TSIG key not found; removing from state")
			return false
		}
		diags.AddError("Couldn't fetch PowerDNS TSIG key", err.Error())
		return false
	}

	m.Name = newSemanticString(tsigNameType, tsigKey.Name)
	m.Algorithm = types.StringValue(tsigKey.Algorithm)
	switch {
	case m.KeyWOVersion.ValueInt64() != 0:
		// Switching to write-only mode must also remove a previously persisted
		// generated or explicit secret, even if the API omits it on this read.
		m.Key = types.StringNull()
	case tsigKey.Key != "":
		m.Key = types.StringValue(tsigKey.Key)
	case m.Key.IsUnknown():
		// Listing omits the secret, but a direct GET returns it. Keep whatever
		// is already in state when the API hands back an empty value.
		m.Key = types.StringNull()
	}
	return true
}

func (r *tsigKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tsigKeyModel
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

func (r *tsigKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, state tsigKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	ctx = tflog.SetField(ctx, "tsigkey_id", plan.ID.ValueString())
	tflog.Debug(ctx, "Updating PowerDNS TSIG key")

	if !plan.Key.Equal(state.Key) || !plan.KeyWOVersion.Equal(state.KeyWOVersion) {
		tsigKey := TSIGKey{Name: plan.Name.ValueString()}
		// In write-only mode, only a version change requests an in-place
		// rotation, even if the configured secret also changed.
		rotate := !plan.Key.Equal(state.Key)
		if plan.KeyWOVersion.ValueInt64() != 0 {
			rotate = !plan.KeyWOVersion.Equal(state.KeyWOVersion)
		}
		if rotate {
			tsigKey.Key = tsigKeySecret(config, plan)
		}

		if _, err := r.clients.PDNS.UpdateTSIGKey(ctx, plan.ID.ValueString(), plan.Algorithm.ValueString(), tsigKey); err != nil {
			resp.Diagnostics.AddError("Error updating PowerDNS TSIG key", err.Error())
			return
		}
	}

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("TSIG key not found after update", plan.ID.ValueString())
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tsigKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tsigKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "tsigkey_id", state.ID.ValueString())
	tflog.Debug(ctx, "Deleting PowerDNS TSIG key")

	if err := r.clients.PDNS.DeleteTSIGKey(ctx, state.ID.ValueString(), state.Algorithm.ValueString()); err != nil {
		if errors.Is(err, ErrNotFound) {
			return
		}
		resp.Diagnostics.AddError("Error deleting PowerDNS TSIG key", fmt.Sprintf("%s", err))
		return
	}
	tflog.Info(ctx, "Deleted PowerDNS TSIG key")
}
