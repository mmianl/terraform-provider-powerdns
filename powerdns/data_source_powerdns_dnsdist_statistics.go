package powerdns

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourcePDNSDNSdistStatistics returns a data source for DNSdist statistics
func dataSourcePDNSDNSdistStatistics() *schema.Resource {
	return &schema.Resource{
		Read: dataSourcePDNSDNSdistStatisticsRead,

		Schema: map[string]*schema.Schema{
			"statistics": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of DNSdist statistics",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the statistic",
						},
						"value": {
							Type:        schema.TypeFloat,
							Computed:    true,
							Description: "Value of the statistic",
						},
						"type": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Type of the statistic",
						},
						"tags": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Tags associated with the statistic",
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func dataSourcePDNSDNSdistStatisticsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	ctx := context.Background()
	tflog.Debug(ctx, "Reading DNSdist statistics")

	statistics, err := client.GetDNSdistStatistics(ctx)
	if err != nil {
		return fmt.Errorf("error reading DNSdist statistics: %s", err)
	}

	// Convert statistics to the format expected by the schema
	statsList := make([]map[string]interface{}, len(statistics))
	for i, stat := range statistics {
		statsList[i] = map[string]interface{}{
			"name":  stat.Name,
			"value": stat.Value,
			"type":  stat.Type,
			"tags":  stat.Tags,
		}
	}

	if err := d.Set("statistics", statsList); err != nil {
		return fmt.Errorf("error setting statistics: %s", err)
	}

	// Use a timestamp as the resource ID since statistics don't have a stable ID
	d.SetId(strconv.FormatInt(getCurrentTimestamp(), 10))

	return nil
}

// getCurrentTimestamp returns the current Unix timestamp
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
