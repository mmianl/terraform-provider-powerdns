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
	_ datasource.DataSource              = &recordDataSource{}
	_ datasource.DataSourceWithConfigure = &recordDataSource{}
)

func newRecordDataSource() datasource.DataSource { return &recordDataSource{} }

type recordDataSource struct {
	clients *ProviderClients
}

type recordDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Disabled types.Bool   `tfsdk:"disabled"`
	Records  types.Set    `tfsdk:"records"`
	Comments types.List   `tfsdk:"comments"`
}

func (d *recordDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (d *recordDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads an existing PowerDNS RRset.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "The record name and type joined by `:::`."},
			"zone": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateFQDN)},
				Description: "The name of the zone containing the record. Must be a fully qualified domain name ending with a trailing dot.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{validateWith(ValidateFQDN)},
				Description: "The fully qualified record name ending with a trailing dot.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The RRset type to look up, for example A, AAAA, MX, or TXT.",
			},
			"ttl": schema.Int64Attribute{
				Computed:    true,
				Description: "The TTL of the RRset in seconds.",
			},
			"disabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether all records in the RRset are disabled in PowerDNS.",
			},
			"records": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Set of record contents in the RRset.",
			},
			"comments": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Ordered list of RRset comments stored in PowerDNS.",
			},
		},
	}
}

func (d *recordDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *recordDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data recordDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := data.Zone.ValueString()
	name := data.Name.ValueString()
	typ := data.Type.ValueString()

	ctx = tflog.SetField(ctx, "zone", zone)
	ctx = tflog.SetField(ctx, "name", name)
	ctx = tflog.SetField(ctx, "type", typ)
	tflog.Info(ctx, "Reading record data source")

	records, err := d.clients.PDNS.ListRecordsInRRSet(ctx, zone, name, typ)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch PowerDNS record", fmt.Sprintf("couldn't fetch PowerDNS record %s %s in zone %s: %s", name, typ, zone, err))
		return
	}
	if len(records) == 0 {
		resp.Diagnostics.AddError("Record not found", fmt.Sprintf("record %s %s not found in zone %s", name, typ, zone))
		return
	}

	recordContents := make([]string, 0, len(records))
	for _, record := range records {
		recordContents = append(recordContents, record.Content)
	}

	rrSet, err := d.clients.PDNS.GetRecordSetByID(ctx, zone, records[0].ID())
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch PowerDNS RRset details", fmt.Sprintf("couldn't fetch PowerDNS RRset details for %s %s in zone %s: %s", name, typ, zone, err))
		return
	}

	recSet, diags := types.SetValueFrom(ctx, types.StringType, recordContents)
	resp.Diagnostics.Append(diags...)
	commentList, diags := types.ListValueFrom(ctx, types.StringType, flattenRRSetComments(commentsFromRRSet(rrSet)))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(records[0].ID())
	data.Name = types.StringValue(records[0].Name)
	data.Type = types.StringValue(records[0].Type)
	data.TTL = types.Int64Value(int64(records[0].TTL))
	data.Disabled = types.BoolValue(rrSetDisabled(records))
	data.Records = recSet
	data.Comments = commentList

	tflog.Info(ctx, "Successfully retrieved PowerDNS record", map[string]any{
		"record_count": len(recordContents),
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func commentsFromRRSet(rrSet *ResourceRecordSet) *[]Comment {
	if rrSet == nil {
		return nil
	}

	return rrSet.Comments
}
