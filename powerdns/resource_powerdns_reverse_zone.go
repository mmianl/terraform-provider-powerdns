package powerdns

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSReverseZone() *schema.Resource {
	return &schema.Resource{
		Create: resourcePDNSReverseZoneCreate,
		Read:   resourcePDNSReverseZoneRead,
		Update: resourcePDNSReverseZoneUpdate,
		Delete: resourcePDNSReverseZoneDelete,
		Exists: resourcePDNSReverseZoneExists,
		Importer: &schema.ResourceImporter{
			State: resourcePDNSReverseZoneImport,
		},

		Schema: map[string]*schema.Schema{
			"cidr": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateCIDR,
				Description:  "The CIDR block for the reverse zone (e.g., '172.16.0.0/16')",
			},
			"kind": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"Master", "Slave"}, false),
				Description:  "The kind of zone (Master or Slave)",
			},
			"nameservers": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringLenBetween(1, 255),
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

func getReverseZoneName(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %s", err)
	}

	if ipnet.IP.To4() != nil {
		// IPv4 reverse zone
		ip := ipnet.IP.To4()
		ones, _ := ipnet.Mask.Size()
		octets := ones / 8

		// For /24 networks, we need to include the third octet in the zone name
		if ones == 24 {
			octets = 3
		}

		// Build the zone name based on the number of octets
		zoneParts := make([]string, octets)
		for i := 0; i < octets; i++ {
			zoneParts[i] = fmt.Sprintf("%d", ip[octets-1-i])
		}
		zone := strings.Join(zoneParts, ".") + ".in-addr.arpa."
		return zone, nil
	} else {
		// IPv6 reverse zone
		ip := ipnet.IP.To16()
		ones, _ := ipnet.Mask.Size()
		nibbles := ones / 4

		// Build the zone name based on the number of nibbles
		zoneParts := make([]string, nibbles)
		for i := 0; i < nibbles; i++ {
			// Get the nibble (4 bits) from the IP address
			byteIndex := i / 2
			nibbleIndex := i % 2
			byte := ip[byteIndex]
			if nibbleIndex == 0 {
				byte = byte >> 4
			} else {
				byte = byte & 0x0F
			}
			zoneParts[nibbles-1-i] = fmt.Sprintf("%x", byte)
		}
		zone := strings.Join(zoneParts, ".") + ".ip6.arpa."
		return zone, nil
	}
}

func expandStringList(configured []interface{}) []string {
	vs := make([]string, 0, len(configured))
	for _, v := range configured {
		vs = append(vs, v.(string))
	}
	return vs
}

func resourcePDNSReverseZoneCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	cidr := d.Get("cidr").(string)
	log.Printf("[INFO] Creating reverse zone for CIDR: %s", cidr)

	zoneName, err := getReverseZoneName(cidr)
	if err != nil {
		return fmt.Errorf("failed to determine zone name: %s", err)
	}
	log.Printf("[INFO] Generated zone name: %s", zoneName)

	// Create the zone
	zone := ZoneInfo{
		Name:        zoneName,
		Kind:        d.Get("kind").(string),
		Nameservers: expandStringList(d.Get("nameservers").([]interface{})),
	}

	createdZone, err := client.CreateZone(zone)
	if err != nil {
		return fmt.Errorf("failed to create reverse zone: %s", err)
	}

	d.SetId(createdZone.Name)
	log.Printf("[INFO] Created reverse zone with ID: %s", d.Id())
	return resourcePDNSReverseZoneRead(d, meta)
}

func resourcePDNSReverseZoneRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	zoneName := d.Id()

	log.Printf("[INFO] Reading reverse zone: %s", zoneName)
	zone, err := client.GetZone(zoneName)
	if err != nil {
		return fmt.Errorf("couldn't fetch zone: %s", err)
	}

	// Check if zone exists by checking if the name is empty
	if zone.Name == "" {
		log.Printf("[WARN] Zone not found, removing from state")
		d.SetId("")
		return nil
	}

	log.Printf("[INFO] Found reverse zone: %s (kind: %s)", zone.Name, zone.Kind)
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

func resourcePDNSReverseZoneUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	zoneName := d.Id()

	if d.HasChange("nameservers") {
		log.Printf("[INFO] Updating nameservers for zone: %s", zoneName)
		// Get the current zone
		zone, err := client.GetZone(zoneName)
		if err != nil {
			return fmt.Errorf("couldn't fetch zone: %s", err)
		}

		// Update nameservers
		zone.Nameservers = expandStringList(d.Get("nameservers").([]interface{}))

		// Create the zone update request
		zoneInfo := ZoneInfoUpd{
			Name:       zoneName,
			Kind:       zone.Kind,
			Account:    zone.Account,
			SoaEditAPI: zone.SoaEditAPI,
		}

		// Update the zone
		err = client.UpdateZone(zoneName, zoneInfo)
		if err != nil {
			return fmt.Errorf("error updating zone: %s", err)
		}

		// Update NS records
		rrSet := ResourceRecordSet{
			Name:       zoneName,
			Type:       "NS",
			TTL:        3600,
			ChangeType: "REPLACE",
			Records:    make([]Record, len(zone.Nameservers)),
		}

		for i, ns := range zone.Nameservers {
			rrSet.Records[i] = Record{
				Content: ns,
				TTL:     3600,
			}
		}

		_, err = client.ReplaceRecordSet(zoneName, rrSet)
		if err != nil {
			return fmt.Errorf("error updating nameserver records: %s", err)
		}
		log.Printf("[INFO] Successfully updated nameservers for zone: %s", zoneName)
	}

	return resourcePDNSReverseZoneRead(d, meta)
}

func resourcePDNSReverseZoneDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	zoneName := d.Id()

	log.Printf("[INFO] Deleting reverse zone: %s", zoneName)
	err := client.DeleteZone(zoneName)
	if err != nil {
		return fmt.Errorf("error deleting zone: %s", err)
	}

	log.Printf("[INFO] Successfully deleted reverse zone: %s", zoneName)
	return nil
}

func resourcePDNSReverseZoneExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*Client)
	zoneName := d.Id()

	log.Printf("[INFO] Checking if reverse zone exists: %s", zoneName)
	zone, err := client.GetZone(zoneName)
	if err != nil {
		return false, fmt.Errorf("error checking zone: %s", err)
	}

	// Check if zone exists by checking if the name is empty
	exists := zone.Name != ""
	if exists {
		log.Printf("[INFO] Reverse zone exists: %s", zoneName)
	} else {
		log.Printf("[INFO] Reverse zone not found: %s", zoneName)
	}
	return exists, nil
}

func resourcePDNSReverseZoneImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*Client)

	// Parse the zone name
	zoneName := d.Id()
	cidr, err := ParseReverseZoneName(zoneName)
	if err != nil {
		return nil, err
	}

	// Verify the zone exists
	zone, err := client.GetZone(zoneName)
	if err != nil {
		return nil, fmt.Errorf("error getting zone: %v", err)
	}

	// Set the resource data
	err = d.Set("name", zoneName)
	if err != nil {
		return nil, fmt.Errorf("error setting name: %s", err)
	}
	err = d.Set("cidr", cidr)
	if err != nil {
		return nil, fmt.Errorf("error setting cidr: %s", err)
	}
	err = d.Set("nameservers", zone.Nameservers)
	if err != nil {
		return nil, fmt.Errorf("error setting nameservers: %s", err)
	}

	return []*schema.ResourceData{d}, nil
}
