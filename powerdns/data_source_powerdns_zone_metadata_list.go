package powerdns

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &zoneMetadataListDataSource{}
	_ datasource.DataSourceWithConfigure = &zoneMetadataListDataSource{}
)

func newZoneMetadataListDataSource() datasource.DataSource { return &zoneMetadataListDataSource{} }

type zoneMetadataListDataSource struct {
	clients *ProviderClients
}

type zoneMetadataListDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Zone    types.String `tfsdk:"zone"`
	Entries types.Set    `tfsdk:"entries"`
}

var zoneMetadataEntryType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"kind":     types.StringType,
	"metadata": types.SetType{ElemType: types.StringType},
}}

func (d *zoneMetadataListDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_metadata_list"
}

func (d *zoneMetadataListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads all PowerDNS zone metadata entries of a zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The zone name, with a trailing dot."},
			"zone": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateZoneName)},
				Description: "Zone name, as FQDN with trailing dot.",
			},
			"entries": schema.SetNestedAttribute{
				Computed:    true,
				Description: "All metadata entries for the zone.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"kind": schema.StringAttribute{
							Computed:    true,
							Description: "Metadata kind name.",
						},
						"metadata": schema.SetAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "Values for the metadata kind.",
						},
					},
				},
			},
		},
	}
}

func (d *zoneMetadataListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *zoneMetadataListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data zoneMetadataListDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := data.Zone.ValueString()

	ctx = tflog.SetField(ctx, "zone", zone)
	tflog.Info(ctx, "Reading all zone metadata data source")

	allMetadata, err := d.clients.PDNS.ListZoneMetadata(ctx, zone)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch zone metadata", fmt.Sprintf("couldn't fetch metadata for zone %s: %s", zone, err))
		return
	}

	entries := make([]attr.Value, 0, len(allMetadata))
	for _, md := range allMetadata {
		values, diags := types.SetValueFrom(ctx, types.StringType, append([]string{}, md.Metadata...))
		resp.Diagnostics.Append(diags...)
		obj, diags := types.ObjectValue(zoneMetadataEntryType.AttrTypes, map[string]attr.Value{
			"kind":     types.StringValue(md.Kind),
			"metadata": values,
		})
		resp.Diagnostics.Append(diags...)
		entries = append(entries, obj)
	}
	set, diags := types.SetValue(zoneMetadataEntryType, entries)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zone)
	data.Entries = set
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
