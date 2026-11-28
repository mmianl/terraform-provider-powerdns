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
	_ datasource.DataSource              = &reverseZoneDataSource{}
	_ datasource.DataSourceWithConfigure = &reverseZoneDataSource{}
)

func newReverseZoneDataSource() datasource.DataSource { return &reverseZoneDataSource{} }

type reverseZoneDataSource struct {
	clients *ProviderClients
}

type reverseZoneDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	CIDR        types.String `tfsdk:"cidr"`
	Kind        types.String `tfsdk:"kind"`
	Nameservers types.List   `tfsdk:"nameservers"`
	Name        types.String `tfsdk:"name"`
}

func (d *reverseZoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_zone"
}

func (d *reverseZoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a PowerDNS reverse zone by CIDR block.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The reverse zone name, with a trailing dot."},
			"cidr": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateCIDR)},
				Description: "The CIDR block for the reverse zone (e.g., '172.16.0.0/16')",
			},
			"kind": schema.StringAttribute{Computed: true, Description: "The kind of zone (Master or Slave)"},
			"nameservers": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of nameservers for this zone",
			},
			"name": schema.StringAttribute{Computed: true, Description: "The computed zone name (e.g., '16.172.in-addr.arpa.')"},
		},
	}
}

func (d *reverseZoneDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *reverseZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data reverseZoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cidr := data.CIDR.ValueString()
	ctx = tflog.SetField(ctx, "cidr", cidr)
	tflog.Info(ctx, "Reading reverse zone data source")

	zoneName, err := GetReverseZoneName(cidr)
	if err != nil {
		resp.Diagnostics.AddError("Failed to determine zone name", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "zone_name", zoneName)
	tflog.Debug(ctx, "Computed reverse zone name from CIDR")

	zone, err := d.clients.PDNS.GetZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch zone", err.Error())
		return
	}

	// Check if zone exists by checking if the name is empty
	if zone.Name == "" {
		resp.Diagnostics.AddError("Reverse zone not found", fmt.Sprintf("reverse zone for CIDR %s not found", cidr))
		return
	}

	tflog.Info(ctx, "Found reverse zone", map[string]interface{}{
		"name": zone.Name,
		"kind": zone.Kind,
	})

	// Read nameservers from NS records
	nameservers, err := d.clients.PDNS.ListRecordsInRRSet(ctx, zoneName, zoneName, "NS")
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch zone nameservers", fmt.Sprintf("couldn't fetch zone %s nameservers from PowerDNS: %s", zoneName, err))
		return
	}
	zoneNameservers := make([]string, 0, len(nameservers))
	for _, ns := range nameservers {
		zoneNameservers = append(zoneNameservers, ns.Content)
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, zoneNameservers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zone.Name)
	data.Name = types.StringValue(zone.Name)
	data.Kind = types.StringValue(zone.Kind)
	data.Nameservers = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
