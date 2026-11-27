package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &zoneMetadataDataSource{}
	_ datasource.DataSourceWithConfigure = &zoneMetadataDataSource{}
)

func newZoneMetadataDataSource() datasource.DataSource { return &zoneMetadataDataSource{} }

type zoneMetadataDataSource struct {
	clients *ProviderClients
}

type zoneMetadataDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Kind     types.String `tfsdk:"kind"`
	Metadata types.Set    `tfsdk:"metadata"`
}

func (d *zoneMetadataDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_metadata"
}

func (d *zoneMetadataDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads one kind of PowerDNS zone metadata.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The zone and metadata kind joined by `:::`."},
			"zone": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateZoneName)},
				Description: "Zone name, as FQDN with trailing dot.",
			},
			"kind": schema.StringAttribute{
				Required:    true,
				Description: "Metadata kind name (for example ALSO-NOTIFY).",
			},
			"metadata": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Values for the requested metadata kind.",
			},
		},
	}
}

func (d *zoneMetadataDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *zoneMetadataDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data zoneMetadataDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := data.Zone.ValueString()
	kind := data.Kind.ValueString()

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "kind", kind)
	tflog.Info(ctx, "Reading zone metadata data source")

	md, err := d.clients.PDNS.GetZoneMetadata(ctx, zone, kind)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch zone metadata", fmt.Sprintf("couldn't fetch metadata %s for zone %s: %s", kind, zone, err))
		return
	}

	values := append([]string{}, md.Metadata...)
	set, diags := types.SetValueFrom(ctx, types.StringType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zoneMetadataID(zone, kind))
	data.Metadata = set
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
