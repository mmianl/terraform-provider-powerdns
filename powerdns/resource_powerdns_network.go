package powerdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSNetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSNetworkCreate,
		ReadContext:   resourcePDNSNetworkRead,
		UpdateContext: resourcePDNSNetworkUpdate,
		DeleteContext: resourcePDNSNetworkDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"network": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.IsCIDR,
			},
			"view": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateViewName,
			},
		},
	}
}

func parseNetworkCIDR(network string) (string, string, error) {
	ip, ipnet, err := net.ParseCIDR(network)
	if err != nil {
		return "", "", fmt.Errorf("invalid network format: %w", err)
	}
	prefixlen, _ := ipnet.Mask.Size()
	return ip.String(), strconv.Itoa(prefixlen), nil
}

func resourcePDNSNetworkCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourcePDNSNetworkUpsert(ctx, d, meta)
}

func resourcePDNSNetworkUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourcePDNSNetworkUpsert(ctx, d, meta)
}

func resourcePDNSNetworkUpsert(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	network := d.Get("network").(string)
	view := d.Get("view").(string)

	ip, prefixlen, err := parseNetworkCIDR(network)
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "network", network)
	ctx = tflog.SetField(ctx, "view", view)
	tflog.Debug(ctx, "Creating or updating PowerDNS network")

	if err := client.PDNS.SetNetwork(ctx, ip, prefixlen, view); err != nil {
		return diag.FromErr(fmt.Errorf("failed to set network %s: %w", network, err))
	}

	d.SetId(network)
	return resourcePDNSNetworkRead(ctx, d, meta)
}

func resourcePDNSNetworkRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	networkCIDR := d.Id()

	ip, prefixlen, err := parseNetworkCIDR(networkCIDR)
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "network", networkCIDR)
	tflog.Debug(ctx, "Reading PowerDNS network")

	network, err := client.PDNS.GetNetwork(ctx, ip, prefixlen)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to get network %s: %w", networkCIDR, err))
	}

	if err := d.Set("network", network.Network); err != nil {
		return diag.FromErr(fmt.Errorf("error setting network: %w", err))
	}
	if err := d.Set("view", network.View); err != nil {
		return diag.FromErr(fmt.Errorf("error setting view: %w", err))
	}

	return nil
}

func resourcePDNSNetworkDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)
	networkCIDR := d.Id()

	ip, prefixlen, err := parseNetworkCIDR(networkCIDR)
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "network", networkCIDR)
	tflog.Debug(ctx, "Deleting PowerDNS network")

	if err := client.PDNS.DeleteNetwork(ctx, ip, prefixlen); err != nil && !errors.Is(err, ErrNotFound) {
		return diag.FromErr(fmt.Errorf("failed to delete network %s: %w", networkCIDR, err))
	}

	return nil
}
