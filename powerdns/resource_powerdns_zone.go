package powerdns

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePDNSZone() *schema.Resource {
	return &schema.Resource{
		Create: resourcePDNSZoneCreate,
		Read:   resourcePDNSZoneRead,
		Update: resourcePDNSZoneUpdate,
		Delete: resourcePDNSZoneDelete,
		Exists: resourcePDNSZoneExists,
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

func resourcePDNSZoneCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	var nameservers []string
	for _, nameserver := range d.Get("nameservers").(*schema.Set).List() {
		nameservers = append(nameservers, nameserver.(string))
	}

	var masters []string
	for _, masterIPPort := range d.Get("masters").(*schema.Set).List() {
		splitIPPort := strings.Split(masterIPPort.(string), ":")
		// if there are more elements
		if len(splitIPPort) > 2 {
			return fmt.Errorf("more than one colon in <ip>:<port> string")
		}
		// when there are exactly 2 elements in list, assume second is port and check the port range
		if len(splitIPPort) == 2 {
			port, err := strconv.Atoi(splitIPPort[1])
			if err != nil {
				return fmt.Errorf("error converting port value in masters atribute")
			}
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid port value in masters atribute")
			}
		}
		// no matter if string contains just IP or IP:port pair, the first element in split list will be IP
		masterIP := splitIPPort[0]
		if net.ParseIP(masterIP) == nil {
			return fmt.Errorf("values in masters list attribute must be valid IPs")
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
			return fmt.Errorf("masters attribute is supported only for Slave kind")
		}
	}

	createdZoneInfo, err := client.CreateZone(zoneInfo)
	if err != nil {
		return err
	}

	d.SetId(createdZoneInfo.ID)
	return resourcePDNSZoneRead(d, meta)
}

func resourcePDNSZoneRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	log.Printf("[DEBUG] Reading PowerDNS Zone: %s", d.Id())
	zoneInfo, err := client.GetZone(d.Id())
	if err != nil {
		return fmt.Errorf("couldn't fetch PowerDNS Zone: %s", err)
	}

	err = d.Set("name", zoneInfo.Name)
	if err != nil {
		return fmt.Errorf("error setting PowerDNS Name: %s", err)
	}

	err = d.Set("kind", zoneInfo.Kind)
	if err != nil {
		return fmt.Errorf("error setting PowerDNS Kind: %s", err)
	}

	err = d.Set("account", zoneInfo.Account)
	if err != nil {
		return fmt.Errorf("error setting PowerDNS Account: %s", err)
	}

	err = d.Set("soa_edit_api", zoneInfo.SoaEditAPI)
	if err != nil {
		return fmt.Errorf("error setting PowerDNS SOA Edit API: %s", err)
	}

	if !strings.EqualFold(zoneInfo.Kind, "Slave") {
		nameservers, err := client.ListRecordsInRRSet(zoneInfo.Name, zoneInfo.Name, "NS")
		if err != nil {
			return fmt.Errorf("couldn't fetch zone %s nameservers from PowerDNS: %v", zoneInfo.Name, err)
		}

		var zoneNameservers []string
		for _, nameserver := range nameservers {
			zoneNameservers = append(zoneNameservers, nameserver.Content)
		}

		err = d.Set("nameservers", zoneNameservers)
		if err != nil {
			return fmt.Errorf("error setting PowerDNS Nameservers: %s", err)
		}
	}

	if strings.EqualFold(zoneInfo.Kind, "Slave") {
		err = d.Set("masters", zoneInfo.Masters)
		if err != nil {
			return fmt.Errorf("error setting PowerDNS Masters: %s", err)
		}
	}

	return nil
}

func resourcePDNSZoneUpdate(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] Updating PowerDNS Zone: %s", d.Id())

	client := meta.(*Client)

	zoneInfo := ZoneInfoUpd{}
	if d.HasChange("kind") || d.HasChange("account") || d.HasChange("soa_edit_api") {
		zoneInfo.Name = d.Get("name").(string)
		zoneInfo.Kind = d.Get("kind").(string)
		zoneInfo.Account = d.Get("account").(string)
		zoneInfo.SoaEditAPI = d.Get("soa_edit_api").(string)

		err := client.UpdateZone(d.Id(), zoneInfo)
		if err != nil {
			return fmt.Errorf("error updating PowerDNS Zone: %s", err)
		}
		return resourcePDNSZoneRead(d, meta)
	}
	return nil
}

func resourcePDNSZoneDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	log.Printf("[INFO] Deleting PowerDNS Zone: %s", d.Id())
	err := client.DeleteZone(d.Id())

	if err != nil {
		return fmt.Errorf("error deleting PowerDNS Zone: %s", err)
	}
	return nil
}

func resourcePDNSZoneExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	log.Printf("[INFO] Checking existence of PowerDNS Zone: %s", d.Id())

	client := meta.(*Client)
	exists, err := client.ZoneExists(d.Id())

	if err != nil {
		return false, fmt.Errorf("error checking PowerDNS Zone: %s", err)
	}
	return exists, nil
}
