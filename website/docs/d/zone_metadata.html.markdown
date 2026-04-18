---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_metadata"
sidebar_current: "docs-powerdns-datasource-zone-metadata"
description: |-
  Reads one specific metadata kind for a PowerDNS zone.
---

# powerdns_zone_metadata

Reads a single zone metadata kind by key.

## Example Usage

```hcl
data "powerdns_zone_metadata" "also_notify" {
  zone = "example.com."
  kind = "ALSO-NOTIFY"
}
```

## Argument Reference

- `zone` - (Required) Zone name as FQDN with trailing dot.
- `kind` - (Required) Metadata kind key (for example `ALSO-NOTIFY`).

## Attribute Reference

- `metadata` - Set of values for this metadata kind.
