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
	_ datasource.DataSource              = &recordSOADataSource{}
	_ datasource.DataSourceWithConfigure = &recordSOADataSource{}
)

func newRecordSOADataSource() datasource.DataSource { return &recordSOADataSource{} }

type recordSOADataSource struct {
	clients *ProviderClients
}

type recordSOADataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Name     types.String `tfsdk:"name"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Disabled types.Bool   `tfsdk:"disabled"`
	Mname    types.String `tfsdk:"mname"`
	Rname    types.String `tfsdk:"rname"`
	Serial   types.Int64  `tfsdk:"serial"`
	Refresh  types.Int64  `tfsdk:"refresh"`
	Retry    types.Int64  `tfsdk:"retry"`
	Expire   types.Int64  `tfsdk:"expire"`
	Minimum  types.Int64  `tfsdk:"minimum"`
}

func (d *recordSOADataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record_soa"
}

func (d *recordSOADataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the SOA record of a PowerDNS zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The record name and type joined by `:::`."},
			"zone": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateZoneName)},
				Description: "The name of the zone containing the SOA record. Must be a fully qualified domain name ending with a trailing dot.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateFQDN)},
				Description: "The name of the SOA record (usually the same as the zone name). Must be a fully qualified domain name ending with a trailing dot.",
			},
			"ttl":      schema.Int64Attribute{Computed: true, Description: "The TTL of the SOA record."},
			"disabled": schema.BoolAttribute{Computed: true, Description: "Whether the SOA record is disabled."},
			"mname":    schema.StringAttribute{Computed: true, Description: "Primary nameserver (MNAME)."},
			"rname":    schema.StringAttribute{Computed: true, Description: "Responsible person email in DNS format (RNAME)."},
			"serial":   schema.Int64Attribute{Computed: true, Description: "SOA serial number."},
			"refresh":  schema.Int64Attribute{Computed: true, Description: "Refresh interval in seconds."},
			"retry":    schema.Int64Attribute{Computed: true, Description: "Retry interval in seconds."},
			"expire":   schema.Int64Attribute{Computed: true, Description: "Expire time in seconds."},
			"minimum":  schema.Int64Attribute{Computed: true, Description: "Minimum TTL (negative caching) in seconds."},
		},
	}
}

func (d *recordSOADataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *recordSOADataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data recordSOADataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := data.Zone.ValueString()
	name := data.Name.ValueString()

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	tflog.Info(ctx, "Reading SOA record data source")

	records, err := d.clients.PDNS.ListRecordsInRRSet(ctx, zone, name, "SOA")
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch SOA record", fmt.Sprintf("couldn't fetch SOA record for %s in zone %s: %s", name, zone, err))
		return
	}
	if len(records) == 0 {
		resp.Diagnostics.AddError("SOA record not found", fmt.Sprintf("SOA record for %s not found in zone %s", name, zone))
		return
	}

	mname, rname, serial, refresh, retry, expire, minimum, err := parseSOAContent(records[0].Content)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing SOA content", err.Error())
		return
	}

	data.ID = types.StringValue(records[0].ID())
	data.TTL = types.Int64Value(int64(records[0].TTL))
	data.Disabled = types.BoolValue(records[0].Disabled)
	data.Mname = types.StringValue(mname)
	data.Rname = types.StringValue(rname)
	data.Serial = types.Int64Value(int64(serial))
	data.Refresh = types.Int64Value(int64(refresh))
	data.Retry = types.Int64Value(int64(retry))
	data.Expire = types.Int64Value(int64(expire))
	data.Minimum = types.Int64Value(int64(minimum))

	tflog.Info(ctx, "Successfully retrieved SOA record")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
