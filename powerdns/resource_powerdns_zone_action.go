package powerdns

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Notify and rectify are operations rather than objects: the server keeps no
// state that can be read back, and destroying one cannot undo it. They are
// modelled the way null_resource models a one-shot action, with a triggers map
// that decides when to run again, so that a plan stays honest about the fact
// that nothing is being tracked on the server.
func resourcePDNSZoneAction(action string, run func(context.Context, *PowerDNSClient, string) error, description string) *schema.Resource {
	return &schema.Resource{
		CreateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return resourcePDNSZoneActionRun(ctx, d, meta, action, run)
		},
		ReadContext:   schema.NoopContext,
		DeleteContext: resourcePDNSZoneActionDelete,

		Description: description,

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateZoneName,
				Description:  "The zone to run the operation on.",
			},

			"triggers": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that cause the operation to run again when they change. " +
					"Without this the operation runs only once, when the resource is first created.",
			},
		},
	}
}

func resourcePDNSZoneActionRun(ctx context.Context, d *schema.ResourceData, meta interface{}, action string, run func(context.Context, *PowerDNSClient, string) error) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	ctx = tflog.SetField(ctx, "zone_name", zone)
	tflog.Debug(ctx, "Running PowerDNS zone operation", map[string]any{"action": action})

	if err := run(ctx, client.PDNS, zone); err != nil {
		if errors.Is(err, ErrNotFound) {
			return diag.FromErr(fmt.Errorf("couldn't run %s: zone %s does not exist", action, zone))
		}
		return diag.FromErr(err)
	}

	d.SetId(zone + ":" + action)
	tflog.Info(ctx, "Ran PowerDNS zone operation", map[string]any{"action": action})
	return nil
}

// resourcePDNSZoneActionDelete only drops the resource from state. There is
// nothing on the server to remove, and a notify or a rectify cannot be undone.
func resourcePDNSZoneActionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}

func resourcePDNSZoneNotify() *schema.Resource {
	return resourcePDNSZoneAction(
		"notify",
		func(ctx context.Context, client *PowerDNSClient, zone string) error {
			return client.NotifyZone(ctx, zone)
		},
		"Sends a NOTIFY for a zone to its slaves. This is an operation rather than a "+
			"tracked object: it runs on create and whenever triggers change, and destroying "+
			"the resource only removes it from state.",
	)
}

func resourcePDNSZoneRectify() *schema.Resource {
	return resourcePDNSZoneAction(
		"rectify",
		func(ctx context.Context, client *PowerDNSClient, zone string) error {
			return client.RectifyZone(ctx, zone)
		},
		"Rectifies a zone, rebuilding its DNSSEC ordering and auth records. This is an "+
			"operation rather than a tracked object: it runs on create and whenever triggers "+
			"change, and destroying the resource only removes it from state.",
	)
}
