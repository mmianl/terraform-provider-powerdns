package powerdns

import (
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func expandStringSet(set *schema.Set) []string {
	values := make([]string, 0, set.Len())
	for _, v := range set.List() {
		values = append(values, v.(string))
	}
	sort.Strings(values)
	return values
}

func getMetadataValues(metadata []ZoneMetadata, kind string) []string {
	for _, entry := range metadata {
		if strings.EqualFold(entry.Kind, kind) {
			values := append([]string(nil), entry.Metadata...)
			sort.Strings(values)
			return values
		}
	}
	return []string{}
}
