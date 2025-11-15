package powerdns

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSZone() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSZoneCreate,
		ReadContext:   resourcePDNSZoneRead,
		UpdateContext: resourcePDNSZoneUpdate,
		DeleteContext: resourcePDNSZoneDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"kind": {
				Type:     schema.TypeString,
				Required: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.EqualFold(old, new)
				},
			},

			"account": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "admin",
				ForceNew:     false,
				ValidateFunc: validation.StringLenBetween(0, 40),
			},

			"nameservers": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},

			"masters": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},

			"soa_edit_api": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
		},
	}
}

func resourcePDNSZoneCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	var nameservers []string
	for _, nameserver := range d.Get("nameservers").(*schema.Set).List() {
		nameservers = append(nameservers, nameserver.(string))
	}

	var masters []string
	for _, masterIPPort := range d.Get("masters").(*schema.Set).List() {
		splitIPPort := strings.Split(masterIPPort.(string), ":")
		// if there are more elements
		if len(splitIPPort) > 2 {
			return diag.FromErr(fmt.Errorf("more than one colon in <ip>:<port> string"))
		}
		// when there are exactly 2 elements in list, assume second is port and check the port range
		if len(splitIPPort) == 2 {
			port, err := strconv.Atoi(splitIPPort[1])
			if err != nil {
				return diag.FromErr(fmt.Errorf("error converting port value in masters attribute"))
			}
			if port < 1 || port > 65535 {
				return diag.FromErr(fmt.Errorf("invalid port value in masters attribute"))
			}
		}
		// first element is IP
		masterIP := splitIPPort[0]
		if net.ParseIP(masterIP) == nil {
			return diag.FromErr(fmt.Errorf("values in masters list attribute must be valid IPs"))
		}
		masters = append(masters, masterIPPort.(string))
	}

	zoneInfo := ZoneInfo{
		Name:        d.Get("name").(string),
		Kind:        d.Get("kind").(string),
		Account:     d.Get("account").(string),
		Nameservers: nameservers,
		SoaEditAPI:  d.Get("soa_edit_api").(string),
	}

	if len(masters) != 0 {
		if strings.EqualFold(zoneInfo.Kind, "Slave") {
			zoneInfo.Masters = masters
		} else {
			return diag.FromErr(fmt.Errorf("masters attribute is supported only for Slave kind"))
		}
	}

	tflog.SetField(ctx, "zone_name", zoneInfo.Name)
	tflog.SetField(ctx, "zone_kind", zoneInfo.Kind)
	tflog.Debug(ctx, "Creating PowerDNS Zone")

	createdZoneInfo, err := client.PDNS.CreateZone(ctx, zoneInfo)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(createdZoneInfo.ID)
	tflog.Info(ctx, "Created PowerDNS Zone", map[string]any{"id": createdZoneInfo.ID})
	return resourcePDNSZoneRead(ctx, d, meta)
}

func resourcePDNSZoneRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Reading PowerDNS Zone")

	zoneInfo, err := client.PDNS.GetZone(ctx, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS Zone: %w", err))
	}

	if zoneInfo.Name == "" {
		tflog.Warn(ctx, "Zone not found; removing from state")
		d.SetId("")
		return nil
	}

	if err := d.Set("name", zoneInfo.Name); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Name: %w", err))
	}
	if err := d.Set("kind", zoneInfo.Kind); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Kind: %w", err))
	}
	if err := d.Set("account", zoneInfo.Account); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS Account: %w", err))
	}
	if err := d.Set("soa_edit_api", zoneInfo.SoaEditAPI); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS SOA Edit API: %w", err))
	}

	// Only manage NS records for non-Slave zones
	if !strings.EqualFold(zoneInfo.Kind, "Slave") {
		nameservers, err := client.PDNS.ListRecordsInRRSet(ctx, zoneInfo.Name, zoneInfo.Name, "NS")
		if err != nil {
			return diag.FromErr(fmt.Errorf("couldn't fetch zone %s nameservers from PowerDNS: %w", zoneInfo.Name, err))
		}

		var zoneNameservers []string
		for _, nameserver := range nameservers {
			zoneNameservers = append(zoneNameservers, nameserver.Content)
		}

		if err := d.Set("nameservers", zoneNameservers); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS Nameservers: %w", err))
		}
	}

	if strings.EqualFold(zoneInfo.Kind, "Slave") {
		if err := d.Set("masters", zoneInfo.Masters); err != nil {
			return diag.FromErr(fmt.Errorf("error setting PowerDNS Masters: %w", err))
		}
	}

	return nil
}

func resourcePDNSZoneUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Updating PowerDNS Zone")

	client := meta.(*ProviderClients)

	zoneInfo := ZoneInfoUpd{}
	if d.HasChange("kind") || d.HasChange("account") || d.HasChange("soa_edit_api") {
		zoneInfo.Name = d.Get("name").(string)
		zoneInfo.Kind = d.Get("kind").(string)
		zoneInfo.Account = d.Get("account").(string)
		zoneInfo.SoaEditAPI = d.Get("soa_edit_api").(string)

		if err := client.PDNS.UpdateZone(ctx, d.Id(), zoneInfo); err != nil {
			return diag.FromErr(fmt.Errorf("error updating PowerDNS Zone: %w", err))
		}
		return resourcePDNSZoneRead(ctx, d, meta)
	}
	return nil
}

func resourcePDNSZoneDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	tflog.SetField(ctx, "zone_id", d.Id())
	tflog.Debug(ctx, "Deleting PowerDNS Zone")

	if err := client.PDNS.DeleteZone(ctx, d.Id()); err != nil {
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS Zone: %w", err))
	}
	tflog.Info(ctx, "Deleted PowerDNS Zone")
	return nil
}
