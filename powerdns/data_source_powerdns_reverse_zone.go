package powerdns

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSReverseZone() *schema.Resource {
	return &schema.Resource{
		Read: dataSourcePDNSReverseZoneRead,

		Schema: map[string]*schema.Schema{
			"cidr": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateCIDR,
				Description:  "The CIDR block for the reverse zone (e.g., '172.16.0.0/16')",
			},
			"kind": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The kind of zone (Master or Slave)",
			},
			"nameservers": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of nameservers for this zone",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The computed zone name (e.g., '16.172.in-addr.arpa.')",
			},
		},
	}
}

func dataSourcePDNSReverseZoneRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	cidr := d.Get("cidr").(string)
	log.Printf("[INFO] Reading reverse zone data source for CIDR: %s", cidr)

	zoneName, err := getReverseZoneName(cidr)
	if err != nil {
		return fmt.Errorf("failed to determine zone name: %s", err)
	}
	log.Printf("[INFO] Generated zone name: %s", zoneName)

	zone, err := client.GetZone(zoneName)
	if err != nil {
		return fmt.Errorf("couldn't fetch zone: %s", err)
	}

	// Check if zone exists by checking if the name is empty
	if zone.Name == "" {
		return fmt.Errorf("reverse zone for CIDR %s not found", cidr)
	}

	log.Printf("[INFO] Found reverse zone: %s (kind: %s)", zone.Name, zone.Kind)

	d.SetId(zone.Name)

	err = d.Set("name", zone.Name)
	if err != nil {
		return fmt.Errorf("error setting name: %s", err)
	}
	err = d.Set("kind", zone.Kind)
	if err != nil {
		return fmt.Errorf("error setting kind: %s", err)
	}

	// Read nameservers from NS records
	nameservers, err := client.ListRecordsInRRSet(zoneName, zoneName, "NS")
	if err != nil {
		return fmt.Errorf("couldn't fetch zone %s nameservers from PowerDNS: %v", zoneName, err)
	}

	var zoneNameservers []string
	for _, nameserver := range nameservers {
		zoneNameservers = append(zoneNameservers, nameserver.Content)
	}

	err = d.Set("nameservers", zoneNameservers)
	if err != nil {
		return fmt.Errorf("error setting nameservers: %s", err)
	}

	return nil
}
