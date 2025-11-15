package powerdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSPTRRecord() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSPTRRecordCreate,
		ReadContext:   resourcePDNSPTRRecordRead,
		DeleteContext: resourcePDNSPTRRecordDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSPTRRecordImport,
		},

		Schema: map[string]*schema.Schema{
			"ip_address": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.Any(validation.IsIPv4Address, validation.IsIPv6Address),
				Description:  "The IP address to create a PTR record for (IPv4 or IPv6).",
			},
			"hostname": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 255),
				Description:  "The hostname to point to.",
			},
			"ttl": {
				Type:         schema.TypeInt,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "The TTL of the PTR record.",
			},
			"reverse_zone": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the reverse zone (e.g., '16.172.in-addr.arpa.' or '8.b.d.0.1.0.0.2.ip6.arpa.').",
			},
		},
	}
}

func resourcePDNSPTRRecordCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ipAddress := d.Get("ip_address").(string)
	hostname := d.Get("hostname").(string)
	ttl := d.Get("ttl").(int)
	reverseZone := d.Get("reverse_zone").(string)

	tflog.SetField(ctx, "ip_address", ipAddress)
	tflog.SetField(ctx, "reverse_zone", reverseZone)
	tflog.Debug(ctx, "Creating PTR record")

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to determine PTR record name: %w", err))
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}

	// Create the PTR record with full FQDN
	rrSet := ResourceRecordSet{
		Name:       ptrName + suffix,
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

	recID, err := client.PDNS.ReplaceRecordSet(ctx, reverseZone, rrSet)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create PTR record: %w", err))
	}

	d.SetId(recID)
	tflog.Info(ctx, "Created PTR record", map[string]any{
		"id":          d.Id(),
		"ptr_name":    rrSet.Name,
		"reverseZone": reverseZone,
	})
	return resourcePDNSPTRRecordRead(ctx, d, meta)
}

func resourcePDNSPTRRecordRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ipAddress := d.Get("ip_address").(string)
	reverseZone := d.Get("reverse_zone").(string)

	tflog.SetField(ctx, "ip_address", ipAddress)
	tflog.SetField(ctx, "reverse_zone", reverseZone)
	tflog.Debug(ctx, "Reading PTR record")

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to determine PTR record name: %w", err))
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}

	records, err := client.PDNS.ListRecordsInRRSet(ctx, reverseZone, ptrName+suffix, "PTR")
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PTR record: %w", err))
	}

	if len(records) == 0 {
		tflog.Warn(ctx, "PTR record not found; removing from state", map[string]any{
			"ptr_name": ptrName + suffix,
		})
		d.SetId("")
		return nil
	}

	tflog.Debug(ctx, "Found PTR record", map[string]any{
		"ptr_name": ptrName + suffix,
		"content":  records[0].Content,
	})

	if err := d.Set("hostname", records[0].Content); err != nil {
		return diag.FromErr(fmt.Errorf("error setting hostname: %w", err))
	}
	if err := d.Set("ttl", records[0].TTL); err != nil {
		return diag.FromErr(fmt.Errorf("error setting ttl: %w", err))
	}
	if err := d.Set("ip_address", ipAddress); err != nil {
		return diag.FromErr(fmt.Errorf("error setting ip_address: %w", err))
	}
	if err := d.Set("reverse_zone", reverseZone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting reverse_zone: %w", err))
	}

	return nil
}

func resourcePDNSPTRRecordDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	ipAddress := d.Get("ip_address").(string)
	reverseZone := d.Get("reverse_zone").(string)

	tflog.SetField(ctx, "ip_address", ipAddress)
	tflog.SetField(ctx, "reverse_zone", reverseZone)
	tflog.Debug(ctx, "Deleting PTR record")

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to determine PTR record name: %w", err))
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}

	if err := client.PDNS.DeleteRecordSet(ctx, reverseZone, ptrName+suffix, "PTR"); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting PTR record: %w", err))
	}

	tflog.Info(ctx, "Successfully deleted PTR record", map[string]any{
		"ptr_name": ptrName + suffix,
	})
	return nil
}

func resourcePDNSPTRRecordImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*ProviderClients)

	tflog.Info(ctx, "Importing PTR record", map[string]any{"id": d.Id()})

	var data map[string]string
	if err := json.Unmarshal([]byte(d.Id()), &data); err != nil {
		return nil, err
	}

	zone, ok := data["zone"]
	if !ok {
		return nil, fmt.Errorf("missing zone in import data")
	}

	recordID, ok := data["id"]
	if !ok {
		return nil, fmt.Errorf("missing id in import data")
	}

	tflog.Debug(ctx, "Fetching PTR record for import", map[string]any{
		"zone":     zone,
		"recordID": recordID,
	})

	records, err := client.PDNS.ListRecordsByID(ctx, zone, recordID)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch PTR record: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("PTR record not found")
	}

	tflog.Debug(ctx, "Found PTR record during import", map[string]any{
		"recordID": recordID,
		"content":  records[0].Content,
	})

	d.SetId(recordID)

	if err := d.Set("reverse_zone", zone); err != nil {
		return nil, fmt.Errorf("error setting reverse_zone: %w", err)
	}
	if err := d.Set("hostname", records[0].Content); err != nil {
		return nil, fmt.Errorf("error setting hostname: %w", err)
	}
	if err := d.Set("ttl", records[0].TTL); err != nil {
		return nil, fmt.Errorf("error setting ttl: %w", err)
	}

	// Extract IP address from PTR record name
	r := strings.Split(recordID, ":::")
	ip, err := ParsePTRRecordName(r[0])
	if err != nil {
		return nil, err
	}
	if err := d.Set("ip_address", ip.String()); err != nil {
		return nil, fmt.Errorf("error setting ip_address: %w", err)
	}

	return []*schema.ResourceData{d}, nil
}
