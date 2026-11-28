package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// cryptokeyKeyTypes lists the key roles accepted by the PowerDNS API.
var cryptokeyKeyTypes = []string{"ksk", "zsk", "csk"}

// cryptokeyAlgorithmAliases maps the short mnemonics and numbers accepted on create to the
// canonical names PowerDNS reports back, so that writing "ecdsa256" does not
// look like a change once the server answers "ECDSAP256SHA256".
var cryptokeyAlgorithmAliases = map[string]string{
	"rsasha1":       "RSASHA1",
	"rsasha256":     "RSASHA256",
	"rsasha512":     "RSASHA512",
	"ecdsa256":      "ECDSAP256SHA256",
	"ecdsa384":      "ECDSAP384SHA384",
	"ed25519":       "ED25519",
	"ed448":         "ED448",
	"ecc-gost":      "ECC-GOST",
	"rsasha1-nsec3": "RSASHA1-NSEC3-SHA1",

	// The DNSSEC algorithm numbers, as listed by IANA.
	"5":  "RSASHA1",
	"7":  "RSASHA1-NSEC3-SHA1",
	"8":  "RSASHA256",
	"10": "RSASHA512",
	"12": "ECC-GOST",
	"13": "ECDSAP256SHA256",
	"14": "ECDSAP384SHA384",
	"15": "ED25519",
	"16": "ED448",
}

// NormalizeCryptokeyAlgorithm resolves an algorithm mnemonic to the canonical
// name used by PowerDNS. Unknown values are upper-cased and returned as-is.
func NormalizeCryptokeyAlgorithm(algorithm string) string {
	if canonical, ok := cryptokeyAlgorithmAliases[strings.ToLower(algorithm)]; ok {
		return canonical
	}
	return strings.ToUpper(algorithm)
}

// cryptokeyAlgorithmForAPI returns the algorithm in a form the PowerDNS API
// accepts. The API knows the names but rejects the numbers, so a number is
// sent as its name. Names are sent as written.
func cryptokeyAlgorithmForAPI(algorithm string) string {
	if _, err := strconv.Atoi(algorithm); err == nil {
		return NormalizeCryptokeyAlgorithm(algorithm)
	}
	return algorithm
}

// cryptokeyAlgorithmType compares the resolved forms, so a mnemonic in the
// configuration and the canonical name the server answers are the same value.
var cryptokeyAlgorithmType = semanticStringType{
	name: "powerdns.CryptokeyAlgorithmType",
	equal: func(prior, next string) bool {
		return NormalizeCryptokeyAlgorithm(prior) == NormalizeCryptokeyAlgorithm(next)
	},
}

// cryptokeyBitsType: algorithms with a fixed key size (the ECDSA and Ed
// curves) have their size chosen by the server, so a configured 0 must not
// look like a change against whatever the server reports.
var cryptokeyBitsType = semanticInt64Type{
	name:  "powerdns.CryptokeyBitsType",
	equal: func(prior, _ int64) bool { return prior == 0 },
}

var (
	_ resource.Resource                = &zoneCryptokeyResource{}
	_ resource.ResourceWithConfigure   = &zoneCryptokeyResource{}
	_ resource.ResourceWithImportState = &zoneCryptokeyResource{}
)

func newZoneCryptokeyResource() resource.Resource { return &zoneCryptokeyResource{} }

type zoneCryptokeyResource struct {
	clients *ProviderClients
}

type zoneCryptokeyModel struct {
	ID        types.String        `tfsdk:"id"`
	Zone      types.String        `tfsdk:"zone"`
	KeyType   semanticStringValue `tfsdk:"keytype"`
	Algorithm semanticStringValue `tfsdk:"algorithm"`
	Bits      semanticInt64Value  `tfsdk:"bits"`
	Content   types.String        `tfsdk:"content"`
	Active    types.Bool          `tfsdk:"active"`
	Published types.Bool          `tfsdk:"published"`
	KeyID     types.Int64         `tfsdk:"key_id"`
	DNSKey    types.String        `tfsdk:"dnskey"`
	DS        types.List          `tfsdk:"ds"`
	CDS       types.List          `tfsdk:"cds"`
}

func (r *zoneCryptokeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_cryptokey"
}

func (r *zoneCryptokeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DNSSEC key of a PowerDNS zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The zone name and the numeric key ID joined by `:`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateZoneName)},
				Description:   "The zone this DNSSEC key belongs to.",
			},
			"keytype": schema.StringAttribute{
				Required:   true,
				CustomType: caseInsensitiveStringType,
				PlanModifiers: []planmodifier.String{requiresReplaceUnlessEquivalent(strings.EqualFold,
					"A different key role forces a new key; a different case does not.")},
				Validators:  []validator.String{stringvalidator.OneOfCaseInsensitive(cryptokeyKeyTypes...)},
				Description: "The role of the key: ksk, zsk or csk.",
			},
			"algorithm": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				CustomType: cryptokeyAlgorithmType,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					requiresReplaceUnlessEquivalent(cryptokeyAlgorithmType.equal,
						"A different algorithm forces a new key; an alias of the same algorithm does not."),
				},
				Description: "The signing algorithm, e.g. ecdsa256 or rsasha256. Changing this forces a new key.",
			},
			"bits": schema.Int64Attribute{
				Optional:   true,
				Computed:   true,
				CustomType: cryptokeyBitsType,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
						// An unknown size cannot be compared yet, so replace to be safe.
						resp.RequiresReplace = req.ConfigValue.IsUnknown() ||
							(!req.ConfigValue.IsNull() && req.ConfigValue.ValueInt64() != 0)
					}, "Changing a configured key size forces a new key.", "Changing a configured key size forces a new key."),
				},
				Description: "The key size in bits. Required for the RSA algorithms, " +
					"derived by the server otherwise. Changing this forces a new key.",
			},
			"content": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "An existing private key in ISC format to import. When omitted, " +
					"PowerDNS generates the key material.",
			},
			"active": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the key is used for signing.",
			},
			"published": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the DNSKEY record is published in the zone.",
			},
			"key_id": schema.Int64Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description:   "The numeric ID PowerDNS assigned to this key.",
			},
			"dnskey": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "The DNSKEY record for this key.",
			},
			"ds": schema.ListAttribute{
				Computed:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "The DS records for this key.",
			},
			"cds": schema.ListAttribute{
				Computed:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "The DS records for this key, filtered by CDS publication settings.",
			},
		},
	}
}

func (r *zoneCryptokeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// cryptokeyID combines the zone and the numeric key ID into a Terraform ID.
func cryptokeyID(zone string, id int) string {
	return zone + ":" + strconv.Itoa(id)
}

// parseCryptokeyID splits a Terraform ID back into its zone and numeric key ID.
func parseCryptokeyID(resourceID string) (string, int, error) {
	zone, rawID, found := strings.Cut(resourceID, ":")
	if !found || zone == "" || rawID == "" {
		return "", 0, fmt.Errorf("unknown cryptokey ID format %q, expected zone:key_id", resourceID)
	}

	id, err := strconv.Atoi(rawID)
	if err != nil {
		return "", 0, fmt.Errorf("invalid cryptokey ID %q: %w", resourceID, err)
	}

	return zone, id, nil
}

func (r *zoneCryptokeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, _, err := parseCryptokeyID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *zoneCryptokeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zoneCryptokeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := plan.Zone.ValueString()
	cryptokey := Cryptokey{
		KeyType:    plan.KeyType.ValueString(),
		Active:     plan.Active.ValueBool(),
		Published:  plan.Published.ValueBool(),
		Algorithm:  cryptokeyAlgorithmForAPI(plan.Algorithm.ValueString()),
		Bits:       int(plan.Bits.ValueInt64()),
		PrivateKey: plan.Content.ValueString(),
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	tflog.Debug(ctx, "Creating PowerDNS cryptokey")

	created, err := r.clients.PDNS.CreateCryptokey(ctx, zone, cryptokey)
	if err != nil {
		resp.Diagnostics.AddError("Error creating PowerDNS cryptokey", err.Error())
		return
	}

	plan.ID = types.StringValue(cryptokeyID(zone, created.ID))
	tflog.Info(ctx, "Created PowerDNS cryptokey", map[string]any{"key_id": created.ID})

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if resp.Diagnostics.HasError() {
			keepCreated(ctx, req.Plan, &resp.State, plan.ID.ValueString(), &resp.Diagnostics)
		} else {
			resp.Diagnostics.AddError("Cryptokey not found after creation", plan.ID.ValueString())
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// refresh loads the key into m, reporting false when it is gone.
func (r *zoneCryptokeyResource) refresh(ctx context.Context, m *zoneCryptokeyModel, diags *diag.Diagnostics) bool {
	zone, id, err := parseCryptokeyID(m.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid id", err.Error())
		return false
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	ctx = tflog.SetField(ctx, "key_id", id)
	tflog.Debug(ctx, "Reading PowerDNS cryptokey")

	cryptokey, err := r.clients.PDNS.GetCryptokey(ctx, zone, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Cryptokey not found; removing from state")
			return false
		}
		diags.AddError("Couldn't fetch PowerDNS cryptokey", err.Error())
		return false
	}

	keytype := cryptokey.KeyType
	// PowerDNS derives roles from the zone's active keys: a KSK/ZSK can appear
	// as CSK, and adding an active ZSK can make a requested CSK appear as KSK.
	// Preserve the previous requested role for these normalizations so refreshes
	// stay stable while configuration changes still force replacement. Imports
	// have no prior role and use the value reported by the server.
	if previous := m.KeyType.ValueString(); previous != "" &&
		(strings.EqualFold(keytype, "csk") ||
			(strings.EqualFold(previous, "csk") && strings.EqualFold(keytype, "ksk"))) {
		keytype = previous
	}

	ds, d := types.ListValueFrom(ctx, types.StringType, append([]string{}, cryptokey.DS...))
	diags.Append(d...)
	cds, d := types.ListValueFrom(ctx, types.StringType, append([]string{}, cryptokey.CDS...))
	diags.Append(d...)
	if diags.HasError() {
		return false
	}

	m.Zone = types.StringValue(zone)
	m.KeyType = newSemanticString(caseInsensitiveStringType, keytype)
	m.Active = types.BoolValue(cryptokey.Active)
	m.Published = types.BoolValue(cryptokey.Published)
	m.Algorithm = newSemanticString(cryptokeyAlgorithmType, cryptokey.Algorithm)
	m.Bits = newSemanticInt64(cryptokeyBitsType, int64(cryptokey.Bits))
	m.KeyID = types.Int64Value(int64(cryptokey.ID))
	m.DNSKey = types.StringValue(cryptokey.DNSKey)
	m.DS = ds
	m.CDS = cds
	return true
}

func (r *zoneCryptokeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state zoneCryptokeyModel
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

func (r *zoneCryptokeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state zoneCryptokeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	zone, id, err := parseCryptokeyID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	ctx = tflog.SetField(ctx, "key_id", id)
	tflog.Debug(ctx, "Updating PowerDNS cryptokey")

	if !plan.Active.Equal(state.Active) || !plan.Published.Equal(state.Published) {
		// PowerDNS rejects an update that leaves out "active", so both flags go
		// out together even when only one of them changed.
		if err := r.clients.PDNS.UpdateCryptokey(ctx, zone, id, plan.Active.ValueBool(), plan.Published.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Error updating PowerDNS cryptokey", err.Error())
			return
		}
	}

	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Cryptokey not found after update", plan.ID.ValueString())
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *zoneCryptokeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state zoneCryptokeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, id, err := parseCryptokeyID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	ctx = tflog.SetField(ctx, "key_id", id)
	tflog.Debug(ctx, "Deleting PowerDNS cryptokey")

	if err := r.clients.PDNS.DeleteCryptokey(ctx, zone, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return
		}
		resp.Diagnostics.AddError("Error deleting PowerDNS cryptokey", err.Error())
		return
	}
	tflog.Info(ctx, "Deleted PowerDNS cryptokey")
}
