package powerdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &ptrRecordResource{}
	_ resource.ResourceWithConfigure   = &ptrRecordResource{}
	_ resource.ResourceWithImportState = &ptrRecordResource{}
)

func newPTRRecordResource() resource.Resource { return &ptrRecordResource{} }

type ptrRecordResource struct {
	clients *ProviderClients
}

type ptrRecordModel struct {
	ID          types.String `tfsdk:"id"`
	IPAddress   types.String `tfsdk:"ip_address"`
	Hostname    types.String `tfsdk:"hostname"`
	TTL         types.Int64  `tfsdk:"ttl"`
	ReverseZone types.String `tfsdk:"reverse_zone"`
}

func validateIPAddress(v interface{}, k string) ([]string, []error) {
	value, _ := v.(string)
	if net.ParseIP(value) == nil {
		return nil, []error{fmt.Errorf("expected %q to be a valid IPv4 or IPv6 address, got: %s", k, value)}
	}
	return nil, nil
}

func (r *ptrRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ptr_record"
}

func (r *ptrRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a PTR record in a reverse zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The PTR record name and type joined by `:::`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ip_address": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(validateIPAddress)},
				Description:   "The IP address to create a PTR record for (IPv4 or IPv6).",
			},
			"hostname": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateFQDN)},
				Description:   "The hostname to point to. Must be a fully qualified domain name ending with a trailing dot.",
			},
			"ttl": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				Description:   "The TTL of the PTR record.",
			},
			"reverse_zone": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validateWith(ValidateFQDN)},
				Description:   "The name of the reverse zone (e.g., '16.172.in-addr.arpa.' or '8.b.d.0.1.0.0.2.ip6.arpa.'). Must be a fully qualified domain name ending with a trailing dot.",
			},
		},
	}
}

func (r *ptrRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.clients = clientsFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// ptrFQDN returns the full PTR record name for an IP address.
func ptrFQDN(ipAddress string) (string, error) {
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return "", fmt.Errorf("failed to determine PTR record name: %w", err)
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}
	return ptrName + suffix, nil
}

func (r *ptrRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing PTR record", map[string]any{"id": req.ID})

	var data map[string]string
	if err := json.Unmarshal([]byte(req.ID), &data); err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}

	zone, ok := data["zone"]
	if !ok {
		resp.Diagnostics.AddError("Invalid import id", "missing zone in import data")
		return
	}
	recordID, ok := data["id"]
	if !ok {
		resp.Diagnostics.AddError("Invalid import id", "missing id in import data")
		return
	}

	// Extract IP address from PTR record name
	parts := strings.Split(recordID, ":::")
	ip, err := ParsePTRRecordName(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), recordID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("reverse_zone"), zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_address"), ip.String())...)
}

func (r *ptrRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ptrRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ipAddress := plan.IPAddress.ValueString()
	hostname := plan.Hostname.ValueString()
	ttl := int(plan.TTL.ValueInt64())
	reverseZone := plan.ReverseZone.ValueString()

	ctx = tflog.SetField(ctx, "ip_address", ipAddress)
	ctx = tflog.SetField(ctx, "reverse_zone", reverseZone)
	tflog.Debug(ctx, "Creating PTR record")

	name, err := ptrFQDN(ipAddress)
	if err != nil {
		resp.Diagnostics.AddError("Failed to determine PTR record name", err.Error())
		return
	}

	// Held across the check and the write below so two resources for the same
	// reverse zone/name created concurrently by this provider can't both
	// observe "does not exist" and both proceed.
	unlock := lockRecordCreate(reverseZone, name, "PTR")
	defer unlock()

	if err := guardRecordOverwrite(ctx, r.clients.PDNS, "powerdns_ptr_record", reverseZone, name, "PTR"); err != nil {
		resp.Diagnostics.AddError("Record already exists", err.Error())
		return
	}

	rrSet := ResourceRecordSet{
		Name:       name,
		Type:       "PTR",
		TTL:        ttl,
		ChangeType: "REPLACE",
		Records: []Record{
			{
				Content: hostname,
				TTL:     ttl,
			},
		},
	}

	recID, err := r.clients.PDNS.ReplaceRecordSet(ctx, reverseZone, rrSet)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create PTR record", err.Error())
		return
	}

	plan.ID = types.StringValue(recID)
	tflog.Info(ctx, "Created PTR record", map[string]any{
		"id":          recID,
		"ptr_name":    rrSet.Name,
		"reverseZone": reverseZone,
	})

	found, err := r.refresh(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Couldn't fetch PTR record", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("PTR record not found after creation", name)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ptrRecordResource) refresh(ctx context.Context, m *ptrRecordModel) (bool, error) {
	ipAddress := m.IPAddress.ValueString()
	reverseZone := m.ReverseZone.ValueString()

	ctx = tflog.SetField(ctx, "ip_address", ipAddress)
	ctx = tflog.SetField(ctx, "reverse_zone", reverseZone)
	tflog.Debug(ctx, "Reading PTR record")

	name, err := ptrFQDN(ipAddress)
	if err != nil {
		return false, err
	}

	records, err := r.clients.PDNS.ListRecordsInRRSet(ctx, reverseZone, name, "PTR")
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Zone no longer exists; removing PTR record from state")
			return false, nil
		}
		return false, fmt.Errorf("couldn't fetch PTR record: %w", err)
	}

	if len(records) == 0 {
		tflog.Warn(ctx, "PTR record not found; removing from state", map[string]any{"ptr_name": name})
		return false, nil
	}

	tflog.Debug(ctx, "Found PTR record", map[string]any{
		"ptr_name": name,
		"content":  records[0].Content,
	})

	m.Hostname = types.StringValue(records[0].Content)
	m.TTL = types.Int64Value(int64(records[0].TTL))
	return true, nil
}

func (r *ptrRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ptrRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.refresh(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Error reading PTR record", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called: every configurable attribute forces replacement.
func (r *ptrRecordResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "powerdns_ptr_record cannot be updated in place.")
}

func (r *ptrRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ptrRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ipAddress := state.IPAddress.ValueString()
	reverseZone := state.ReverseZone.ValueString()

	ctx = tflog.SetField(ctx, "ip_address", ipAddress)
	ctx = tflog.SetField(ctx, "reverse_zone", reverseZone)
	tflog.Debug(ctx, "Deleting PTR record")

	name, err := ptrFQDN(ipAddress)
	if err != nil {
		resp.Diagnostics.AddError("Failed to determine PTR record name", err.Error())
		return
	}

	if err := r.clients.PDNS.DeleteRecordSet(ctx, reverseZone, name, "PTR"); err != nil {
		resp.Diagnostics.AddError("Error deleting PTR record", err.Error())
		return
	}

	tflog.Info(ctx, "Successfully deleted PTR record", map[string]any{"ptr_name": name})
}
