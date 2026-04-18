---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_metadata"
sidebar_current: "docs-powerdns-resource-zone-metadata"
description: |-
  Manages per-zone metadata entries for a PowerDNS authoritative zone.
---

# powerdns_zone_metadata

Manages one metadata kind for one zone via the PowerDNS metadata API.

## Example Usage

```hcl
resource "powerdns_zone" "example" {
  name = "example.com."
  kind = "Master"
}

resource "powerdns_zone_metadata" "also_notify" {
  zone     = powerdns_zone.example.name
  kind     = "ALSO-NOTIFY"
  metadata = ["192.0.2.10", "192.0.2.11:5300"]
}

resource "powerdns_zone_metadata" "allow_axfr_from" {
  zone     = powerdns_zone.example.name
  kind     = "ALLOW-AXFR-FROM"
  metadata = ["AUTO-NS", "2001:db8::/48"]
}
```

## Argument Reference

The following arguments are supported:

- `zone` - (Required, Forces new resource) Zone name, as FQDN with trailing dot (for example `"example.com."`).
- `kind` - (Required, Forces new resource) Metadata kind name exactly as expected by PowerDNS (for example `ALSO-NOTIFY`).
- `metadata` - (Required) Set of values for the metadata kind.

## Importing

Import format is `<zone>:::<kind>`.

Example:

```bash
terraform import powerdns_zone_metadata.also_notify 'example.com.:::ALSO-NOTIFY'
```
