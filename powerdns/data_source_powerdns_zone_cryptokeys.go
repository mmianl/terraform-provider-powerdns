package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &zoneCryptokeysDataSource{}
	_ datasource.DataSourceWithConfigure = &zoneCryptokeysDataSource{}
)

func newZoneCryptokeysDataSource() datasource.DataSource { return &zoneCryptokeysDataSource{} }

type zoneCryptokeysDataSource struct {
	clients *ProviderClients
}

type zoneCryptokeysDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Zone       types.String `tfsdk:"zone"`
	Cryptokeys types.List   `tfsdk:"cryptokeys"`
}

var cryptokeyObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"key_id":    types.Int64Type,
	"keytype":   types.StringType,
	"active":    types.BoolType,
	"published": types.BoolType,
	"algorithm": types.StringType,
	"bits":      types.Int64Type,
	"dnskey":    types.StringType,
	"ds":        types.ListType{ElemType: types.StringType},
	"cds":       types.ListType{ElemType: types.StringType},
}}

func (d *zoneCryptokeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_cryptokeys"
}

func (d *zoneCryptokeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the DNSSEC keys of a PowerDNS zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The zone name, with a trailing dot."},
			"zone": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateZoneName)},
				Description: "The zone whose DNSSEC keys should be retrieved.",
			},
			"cryptokeys": schema.ListNestedAttribute{
				Computed:    true,
				Description: "All DNSSEC keys configured for the zone",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key_id":    schema.Int64Attribute{Computed: true, Description: "The numeric ID PowerDNS assigned to this key"},
						"keytype":   schema.StringAttribute{Computed: true, Description: "The role of the key: ksk, zsk or csk"},
						"active":    schema.BoolAttribute{Computed: true, Description: "Whether the key is used for signing"},
						"published": schema.BoolAttribute{Computed: true, Description: "Whether the DNSKEY record is published in the zone"},
						"algorithm": schema.StringAttribute{Computed: true, Description: "The signing algorithm of the key"},
						"bits":      schema.Int64Attribute{Computed: true, Description: "The key size in bits"},
						"dnskey":    schema.StringAttribute{Computed: true, Description: "The DNSKEY record for this key"},
						"ds": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "The DS records for this key",
						},
						"cds": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "The DS records for this key, filtered by CDS publication settings",
						},
					},
				},
			},
		},
	}
}

func (d *zoneCryptokeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *zoneCryptokeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data zoneCryptokeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := data.Zone.ValueString()
	ctx = tflog.SetField(ctx, "zone_name", zone)
	tflog.Info(ctx, "Reading zone cryptokeys data source")

	cryptokeys, err := d.clients.PDNS.ListCryptokeys(ctx, zone)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			resp.Diagnostics.AddError("Zone not found", fmt.Sprintf("zone %s not found", zone))
			return
		}
		resp.Diagnostics.AddError("Could not fetch cryptokeys", fmt.Sprintf("couldn't fetch cryptokeys for zone %s: %s", zone, err))
		return
	}

	keys := make([]attr.Value, 0, len(cryptokeys))
	for _, c := range cryptokeys {
		ds, diags := types.ListValueFrom(ctx, types.StringType, append([]string{}, c.DS...))
		resp.Diagnostics.Append(diags...)
		cds, diags := types.ListValueFrom(ctx, types.StringType, append([]string{}, c.CDS...))
		resp.Diagnostics.Append(diags...)
		obj, diags := types.ObjectValue(cryptokeyObjectType.AttrTypes, map[string]attr.Value{
			"key_id":    types.Int64Value(int64(c.ID)),
			"keytype":   types.StringValue(c.KeyType),
			"active":    types.BoolValue(c.Active),
			"published": types.BoolValue(c.Published),
			"algorithm": types.StringValue(c.Algorithm),
			"bits":      types.Int64Value(int64(c.Bits)),
			"dnskey":    types.StringValue(c.DNSKey),
			"ds":        ds,
			"cds":       cds,
		})
		resp.Diagnostics.Append(diags...)
		keys = append(keys, obj)
	}
	list, diags := types.ListValue(cryptokeyObjectType, keys)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zone)
	data.Cryptokeys = list

	tflog.Info(ctx, "Successfully retrieved zone cryptokeys", map[string]interface{}{
		"key_count": len(keys),
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
