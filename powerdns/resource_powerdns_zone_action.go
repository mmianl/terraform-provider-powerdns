package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &zoneActionResource{}
	_ resource.ResourceWithConfigure = &zoneActionResource{}
)

// Notify and rectify are operations rather than objects: the server keeps no
// state that can be read back, and destroying one cannot undo it. They are
// modelled the way null_resource models a one-shot action, with a triggers map
// that decides when to run again, so that a plan stays honest about the fact
// that nothing is being tracked on the server.
type zoneActionResource struct {
	clients     *ProviderClients
	action      string
	description string
	run         func(context.Context, *PowerDNSClient, string) error
}

type zoneActionModel struct {
	ID       types.String `tfsdk:"id"`
	Zone     types.String `tfsdk:"zone"`
	Triggers types.Map    `tfsdk:"triggers"`
}

func newZoneNotifyResource() resource.Resource {
	return &zoneActionResource{
		action: "notify",
		run: func(ctx context.Context, client *PowerDNSClient, zone string) error {
			return client.NotifyZone(ctx, zone)
		},
		description: "Sends a NOTIFY for a zone to its slaves. This is an operation rather than a " +
			"tracked object: it runs on create and whenever triggers change, and destroying " +
			"the resource only removes it from state.",
	}
}

func newZoneRectifyResource() resource.Resource {
	return &zoneActionResource{
		action: "rectify",
		run: func(ctx context.Context, client *PowerDNSClient, zone string) error {
			return client.RectifyZone(ctx, zone)
		},
		description: "Rectifies a zone, rebuilding its DNSSEC ordering and auth records. This is an " +
			"operation rather than a tracked object: it runs on create and whenever triggers " +
			"change, and destroying the resource only removes it from state.",
	}
}

func (r *zoneActionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone_" + r.action
}

func (r *zoneActionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: r.description,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The zone name, with a trailing dot.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateZoneName)},
				Description:   "The zone to run the operation on.",
			},
			"triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
				Description: "Arbitrary values that cause the operation to run again when they change. " +
					"Without this the operation runs only once, when the resource is first created.",
			},
		},
	}
}

func (r *zoneActionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *zoneActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan zoneActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone := plan.Zone.ValueString()
	ctx = tflog.SetField(ctx, "zone_name", zone)
	tflog.Debug(ctx, "Running PowerDNS zone operation", map[string]any{"action": r.action})

	if err := r.run(ctx, r.clients.PDNS, zone); err != nil {
		if errors.Is(err, ErrNotFound) {
			resp.Diagnostics.AddError("Zone not found", fmt.Sprintf("couldn't run %s: zone %s does not exist", r.action, zone))
			return
		}
		resp.Diagnostics.AddError("Zone operation failed", err.Error())
		return
	}

	plan.ID = types.StringValue(zone + ":" + r.action)
	tflog.Info(ctx, "Ran PowerDNS zone operation", map[string]any{"action": r.action})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the stored state: there is nothing on the server to read back.
func (r *zoneActionResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every configurable attribute forces replacement.
func (r *zoneActionResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "zone operations cannot be updated in place.")
}

// Delete only drops the resource from state. There is nothing on the server to
// remove, and a notify or a rectify cannot be undone.
func (r *zoneActionResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
