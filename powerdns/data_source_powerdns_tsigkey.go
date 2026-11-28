package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &tsigKeyDataSource{}
	_ datasource.DataSourceWithConfigure = &tsigKeyDataSource{}
)

func newTSIGKeyDataSource() datasource.DataSource { return &tsigKeyDataSource{} }

type tsigKeyDataSource struct {
	clients *ProviderClients
}

type tsigKeyDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Algorithm types.String `tfsdk:"algorithm"`
	Key       types.String `tfsdk:"key"`
}

func (d *tsigKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tsigkey"
}

func (d *tsigKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a PowerDNS TSIG key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the TSIG key to retrieve, e.g. \"mykey.\".",
			},
			"name":      schema.StringAttribute{Computed: true, Description: "The name of the TSIG key"},
			"algorithm": schema.StringAttribute{Computed: true, Description: "The TSIG algorithm of the key"},
			"key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The base64 encoded secret of the key",
			},
		},
	}
}

func (d *tsigKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *tsigKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data tsigKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyID := data.ID.ValueString()
	ctx = tflog.SetField(ctx, "tsigkey_id", keyID)
	tflog.Info(ctx, "Reading TSIG key data source")

	tsigKey, err := d.clients.PDNS.GetTSIGKey(ctx, keyID, "")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			resp.Diagnostics.AddError("TSIG key not found", fmt.Sprintf("TSIG key %s not found", keyID))
			return
		}
		resp.Diagnostics.AddError("Could not fetch TSIG key", fmt.Sprintf("couldn't fetch TSIG key %s: %s", keyID, err))
		return
	}

	data.ID = types.StringValue(tsigKey.ID)
	data.Name = types.StringValue(tsigKey.Name)
	data.Algorithm = types.StringValue(tsigKey.Algorithm)
	data.Key = types.StringValue(tsigKey.Key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
