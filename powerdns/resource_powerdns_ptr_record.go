package powerdns

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSPTRRecord() *schema.Resource {
	return &schema.Resource{
		Create: resourcePDNSPTRRecordCreate,
		Read:   resourcePDNSPTRRecordRead,
		Delete: resourcePDNSPTRRecordDelete,
		Exists: resourcePDNSPTRRecordExists,
		Importer: &schema.ResourceImporter{
			State: resourcePDNSPTRRecordImport,
		},

		Schema: map[string]*schema.Schema{
			"ip_address": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.Any(validation.IsIPv4Address, validation.IsIPv6Address),
				Description:  "The IP address to create a PTR record for (IPv4 or IPv6)",
			},

			"hostname": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 255),
				Description:  "The hostname to point to",
			},

			"ttl": {
				Type:         schema.TypeInt,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "The TTL of the PTR record",
			},

			"reverse_zone": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the reverse zone (e.g., '16.172.in-addr.arpa.' or '8.b.d.0.1.0.0.2.ip6.arpa.')",
			},
		},
	}
}

func resourcePDNSPTRRecordCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	ipAddress := d.Get("ip_address").(string)
	hostname := d.Get("hostname").(string)
	ttl := d.Get("ttl").(int)
	reverseZone := d.Get("reverse_zone").(string)

	log.Printf("[INFO] Creating PTR record for IP address: %s", ipAddress)

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return fmt.Errorf("failed to determine PTR record name: %s", err)
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

	recID, err := client.ReplaceRecordSet(reverseZone, rrSet)
	if err != nil {
		return fmt.Errorf("failed to create PTR record: %s", err)
	}

	d.SetId(recID)
	log.Printf("[INFO] Created PTR record with ID: %s", d.Id())
	return resourcePDNSPTRRecordRead(d, meta)
}

func resourcePDNSPTRRecordRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	ipAddress := d.Get("ip_address").(string)
	reverseZone := d.Get("reverse_zone").(string)

	log.Printf("[INFO] Reading PTR record for IP: %s in zone: %s", ipAddress, reverseZone)

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return fmt.Errorf("failed to determine PTR record name: %s", err)
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}

	records, err := client.ListRecordsInRRSet(reverseZone, ptrName+suffix, "PTR")
	if err != nil {
		return fmt.Errorf("couldn't fetch PTR record: %s", err)
	}

	if len(records) == 0 {
		log.Printf("[WARN] PTR record not found, removing from state")
		d.SetId("")
		return nil
	}

	log.Printf("[INFO] Found PTR record: %s -> %s", ptrName+suffix, records[0].Content)

	err = d.Set("hostname", records[0].Content)
	if err != nil {
		return fmt.Errorf("error setting hostname: %s", err)
	}

	err = d.Set("ttl", records[0].TTL)
	if err != nil {
		return fmt.Errorf("error setting TTL: %s", err)
	}

	err = d.Set("ip_address", ipAddress)
	if err != nil {
		return fmt.Errorf("error setting ip_address: %s", err)
	}

	err = d.Set("reverse_zone", reverseZone)
	if err != nil {
		return fmt.Errorf("error setting reverse_zone: %s", err)
	}

	return nil
}

func resourcePDNSPTRRecordDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	ipAddress := d.Get("ip_address").(string)
	reverseZone := d.Get("reverse_zone").(string)

	log.Printf("[INFO] Deleting PTR record for IP: %s from zone: %s", ipAddress, reverseZone)

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return fmt.Errorf("failed to determine PTR record name: %s", err)
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}

	err = client.DeleteRecordSet(reverseZone, ptrName+suffix, "PTR")
	if err != nil {
		return fmt.Errorf("error deleting PTR record: %s", err)
	}

	log.Printf("[INFO] Successfully deleted PTR record")
	return nil
}

func resourcePDNSPTRRecordExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*Client)

	ipAddress := d.Get("ip_address").(string)
	reverseZone := d.Get("reverse_zone").(string)

	log.Printf("[INFO] Checking if PTR record exists for IP: %s in zone: %s", ipAddress, reverseZone)

	// Get the PTR record name
	ptrName, err := GetPTRRecordName(ipAddress)
	if err != nil {
		return false, fmt.Errorf("failed to determine PTR record name: %s", err)
	}

	// Determine the correct suffix based on IP version
	suffix := ".in-addr.arpa."
	if net.ParseIP(ipAddress).To4() == nil {
		suffix = ".ip6.arpa."
	}

	exists, err := client.RecordExists(reverseZone, ptrName+suffix, "PTR")
	if err != nil {
		return false, fmt.Errorf("error checking PTR record: %s", err)
	}

	if exists {
		log.Printf("[INFO] PTR record exists")
	} else {
		log.Printf("[INFO] PTR record not found")
	}
	return exists, nil
}

func resourcePDNSPTRRecordImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*Client)

	log.Printf("[INFO] Importing PTR record with ID: %s", d.Id())

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

	log.Printf("[INFO] Importing PTR record %s from zone %s", recordID, zone)

	records, err := client.ListRecordsByID(zone, recordID)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch PTR record: %s", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("PTR record not found")
	}

	log.Printf("[INFO] Found PTR record: %s -> %s", recordID, records[0].Content)

	d.SetId(recordID)

	err = d.Set("reverse_zone", zone)
	if err != nil {
		return nil, fmt.Errorf("error setting reverse_zone: %s", err)
	}

	err = d.Set("hostname", records[0].Content)
	if err != nil {
		return nil, fmt.Errorf("error setting hostname: %s", err)
	}

	err = d.Set("ttl", records[0].TTL)
	if err != nil {
		return nil, fmt.Errorf("error setting ttl: %s", err)
	}

	// Extract IP address from PTR record name
	r := strings.Split(recordID, ":::")
	ip, err := ParsePTRRecordName(r[0])
	if err != nil {
		return nil, err
	}
	err = d.Set("ip_address", ip.String())
	if err != nil {
		return nil, fmt.Errorf("error setting ip_address: %s", err)
	}

	return []*schema.ResourceData{d}, nil
}
