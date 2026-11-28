package powerdns

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &zoneDataSource{}
	_ datasource.DataSourceWithConfigure = &zoneDataSource{}
)

func newZoneDataSource() datasource.DataSource { return &zoneDataSource{} }

type zoneDataSource struct {
	clients *ProviderClients
}

type zoneDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Kind       types.String `tfsdk:"kind"`
	Account    types.String `tfsdk:"account"`
	Masters    types.Set    `tfsdk:"masters"`
	SoaEditAPI types.String `tfsdk:"soa_edit_api"`
	Catalog    types.String `tfsdk:"catalog"`
	SoaEdit    types.String `tfsdk:"soa_edit"`
	DNSSec     types.Bool   `tfsdk:"dnssec"`
	APIRectify types.Bool   `tfsdk:"api_rectify"`
	Serial     types.Int64  `tfsdk:"serial"`

	NotifiedSerial   types.Int64 `tfsdk:"notified_serial"`
	EditedSerial     types.Int64 `tfsdk:"edited_serial"`
	LastCheck        types.Int64 `tfsdk:"last_check"`
	Records          types.List  `tfsdk:"records"`
	MasterTsigKeyIDs types.Set   `tfsdk:"master_tsig_key_ids"`
	SlaveTsigKeyIDs  types.Set   `tfsdk:"slave_tsig_key_ids"`
}

var zoneRecordObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":     types.StringType,
	"type":     types.StringType,
	"content":  types.StringType,
	"ttl":      types.Int64Type,
	"disabled": types.BoolType,
}}

func (d *zoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (d *zoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a PowerDNS zone and its records.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The zone name, with a trailing dot."},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateZoneName)},
				Description: "The name of the zone to retrieve. Must be a fully qualified domain name ending with a trailing dot or zone variant.",
			},
			"kind":    schema.StringAttribute{Computed: true, Description: "The kind of zone (Master, Slave, etc.)"},
			"account": schema.StringAttribute{Computed: true, Description: "The account associated with the zone"},
			"masters": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of master servers for this zone (Slave zones only)",
			},
			"soa_edit_api": schema.StringAttribute{Computed: true, Description: "SOA edit API setting"},
			"catalog":      schema.StringAttribute{Computed: true, Description: "The catalog zone this zone belongs to"},
			"soa_edit":     schema.StringAttribute{Computed: true, Description: "SOA edit setting applied when serving the zone"},
			"dnssec":       schema.BoolAttribute{Computed: true, Description: "Whether DNSSEC is enabled for this zone"},
			"api_rectify":  schema.BoolAttribute{Computed: true, Description: "Whether the zone is rectified automatically after API changes"},
			"serial":       schema.Int64Attribute{Computed: true, Description: "The current serial of the zone"},
			"notified_serial": schema.Int64Attribute{
				Computed: true, Description: "The last serial this zone was notified for",
			},
			"edited_serial": schema.Int64Attribute{
				Computed: true, Description: "The SOA serial after applying the SOA edit rule",
			},
			"last_check": schema.Int64Attribute{
				Computed: true, Description: "Timestamp of the last freshness check (Slave zones only)",
			},
			"master_tsig_key_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "TSIG key IDs allowed to authenticate AXFR requests for this zone.",
			},
			"slave_tsig_key_ids": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "TSIG key IDs used to sign requests to this zone's primary server.",
			},
			"records": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of all records in the zone",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":     schema.StringAttribute{Computed: true, Description: "The name of the record"},
						"type":     schema.StringAttribute{Computed: true, Description: "The type of the record (A, AAAA, CNAME, etc.)"},
						"content":  schema.StringAttribute{Computed: true, Description: "The content of the record"},
						"ttl":      schema.Int64Attribute{Computed: true, Description: "The TTL of the record"},
						"disabled": schema.BoolAttribute{Computed: true, Description: "Whether the record is disabled"},
					},
				},
			},
		},
	}
}

func (d *zoneDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *zoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data zoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.Name.ValueString()
	ctx = tflog.SetField(ctx, "zone_name", zoneName)
	tflog.Info(ctx, "Reading zone data source")

	zone, err := d.clients.PDNS.GetZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch zone", fmt.Sprintf("couldn't fetch zone %s: %s", zoneName, err))
		return
	}
	if zone.Name == "" {
		resp.Diagnostics.AddError("Zone not found", fmt.Sprintf("zone %s not found", zoneName))
		return
	}

	ctx = tflog.SetField(ctx, "kind", zone.Kind)
	tflog.Info(ctx, "Found zone")

	data.ID = types.StringValue(zone.Name)
	data.Name = types.StringValue(zone.Name)
	data.Kind = types.StringValue(zone.Kind)
	data.Account = types.StringValue(zone.Account)
	data.SoaEditAPI = types.StringValue(zone.SoaEditAPI)
	data.Catalog = types.StringValue(zone.Catalog)
	data.SoaEdit = types.StringValue(zone.SoaEdit)
	data.DNSSec = types.BoolValue(zone.DNSSec)
	data.APIRectify = types.BoolNull()
	if zone.APIRectify != nil {
		data.APIRectify = types.BoolValue(*zone.APIRectify)
	}
	data.Serial = types.Int64Value(zone.Serial)
	data.NotifiedSerial = types.Int64Value(zone.NotifiedSerial)
	data.EditedSerial = types.Int64Value(zone.EditedSerial)
	data.LastCheck = types.Int64Value(zone.LastCheck)

	masterIDs, diags := types.SetValueFrom(ctx, types.StringType, append([]string{}, zone.MasterTsigKeyIDs...))
	resp.Diagnostics.Append(diags...)
	slaveIDs, diags := types.SetValueFrom(ctx, types.StringType, append([]string{}, zone.SlaveTsigKeyIDs...))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.MasterTsigKeyIDs = masterIDs
	data.SlaveTsigKeyIDs = slaveIDs

	// Masters only apply to Slave zones.
	data.Masters = types.SetNull(types.StringType)
	if strings.EqualFold(zone.Kind, "Slave") {
		set, diags := types.SetValueFrom(ctx, types.StringType, zone.Masters)
		resp.Diagnostics.Append(diags...)
		data.Masters = set
	}

	allRecords, err := d.clients.PDNS.ListRecords(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch zone records", fmt.Sprintf("couldn't fetch records for zone %s: %s", zoneName, err))
		return
	}

	records := make([]attr.Value, 0, len(allRecords))
	for _, r := range allRecords {
		obj, diags := types.ObjectValue(zoneRecordObjectType.AttrTypes, map[string]attr.Value{
			"name":     types.StringValue(r.Name),
			"type":     types.StringValue(r.Type),
			"content":  types.StringValue(r.Content),
			"ttl":      types.Int64Value(int64(r.TTL)),
			"disabled": types.BoolValue(r.Disabled),
		})
		resp.Diagnostics.Append(diags...)
		records = append(records, obj)
	}
	list, diags := types.ListValue(zoneRecordObjectType, records)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = list

	tflog.Info(ctx, "Successfully retrieved zone records", map[string]interface{}{
		"record_count": len(records),
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
