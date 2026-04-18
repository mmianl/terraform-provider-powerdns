---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_metadata_list"
sidebar_current: "docs-powerdns-datasource-zone-metadata-list"
description: |-
  Reads all metadata entries for a PowerDNS zone.
---

# powerdns_zone_metadata_list

Reads all metadata entries for a zone.

## Example Usage

```hcl
data "powerdns_zone_metadata_list" "all" {
  zone = "example.com."
}

output "metadata_entries" {
  value = data.powerdns_zone_metadata_list.all.entries
}
```

## Argument Reference

- `zone` - (Required) Zone name as FQDN with trailing dot.

## Attribute Reference

- `entries` - Set of metadata entries. Each entry has:
  - `kind` - Metadata kind key.
  - `metadata` - Set of values for that key.
