package powerdns

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePDNSZone() *schema.Resource {
	return &schema.Resource{
		Read: dataSourcePDNSZoneRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the zone to retrieve",
			},
			"kind": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The kind of zone (Master, Slave, etc.)",
			},
			"account": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The account associated with the zone",
			},
			"nameservers": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of nameservers for this zone",
			},
			"masters": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "List of master servers for this zone (Slave zones only)",
			},
			"soa_edit_api": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "SOA edit API setting",
			},
			"records": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the record",
						},
						"type": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The type of the record (A, AAAA, CNAME, etc.)",
						},
						"content": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The content of the record",
						},
						"ttl": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The TTL of the record",
						},
						"disabled": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the record is disabled",
						},
					},
				},
				Description: "List of all records in the zone",
			},
		},
	}
}

func dataSourcePDNSZoneRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	zoneName := d.Get("name").(string)
	log.Printf("[INFO] Reading zone data source for zone: %s", zoneName)

	// Get the zone information
	zone, err := client.GetZone(zoneName)
	if err != nil {
		return fmt.Errorf("couldn't fetch zone %s: %s", zoneName, err)
	}

	// Check if zone exists
	if zone.Name == "" {
		return fmt.Errorf("zone %s not found", zoneName)
	}

	log.Printf("[INFO] Found zone: %s (kind: %s)", zone.Name, zone.Kind)

	// Set zone information
	d.SetId(zone.Name)

	err = d.Set("name", zone.Name)
	if err != nil {
		return fmt.Errorf("error setting zone name: %s", err)
	}

	err = d.Set("kind", zone.Kind)
	if err != nil {
		return fmt.Errorf("error setting zone kind: %s", err)
	}

	err = d.Set("account", zone.Account)
	if err != nil {
		return fmt.Errorf("error setting zone account: %s", err)
	}

	err = d.Set("soa_edit_api", zone.SoaEditAPI)
	if err != nil {
		return fmt.Errorf("error setting zone SOA edit API: %s", err)
	}

	// Set nameservers for non-Slave zones
	if !strings.EqualFold(zone.Kind, "Slave") {
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
			return fmt.Errorf("error setting zone nameservers: %s", err)
		}
	}

	// Set masters for Slave zones
	if strings.EqualFold(zone.Kind, "Slave") {
		err = d.Set("masters", zone.Masters)
		if err != nil {
			return fmt.Errorf("error setting zone masters: %s", err)
		}
	}

	// Get all records in the zone and link them to the zone data
	allRecords, err := client.ListRecords(zoneName)
	if err != nil {
		return fmt.Errorf("couldn't fetch records for zone %s: %s", zoneName, err)
	}

	// Convert records to the schema format
	var records []map[string]interface{}
	for _, record := range allRecords {
		recordMap := map[string]interface{}{
			"name":     record.Name,
			"type":     record.Type,
			"content":  record.Content,
			"ttl":      record.TTL,
			"disabled": record.Disabled,
		}
		records = append(records, recordMap)
	}

	err = d.Set("records", records)
	if err != nil {
		return fmt.Errorf("error setting zone records: %s", err)
	}

	log.Printf("[INFO] Successfully retrieved zone %s with %d records", zoneName, len(records))
	return nil
}
