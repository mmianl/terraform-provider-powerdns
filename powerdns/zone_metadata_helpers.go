package powerdns

import (
	"sort"
	"strings"
)

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
