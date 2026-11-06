---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_rectify"
sidebar_current: "docs-powerdns-resource-zone-rectify"
description: |-
  Rectifies a zone, rebuilding its DNSSEC ordering and auth records.
---

# powerdns_zone_rectify

Rectifies a zone, rebuilding its DNSSEC ordering and auth records. This is the same operation as `pdnsutil rectify-zone`, and is mainly useful for zones that are not rectified automatically.

~> **Note:** This is an operation rather than a tracked object. It runs when the resource is created and whenever `triggers` change, nothing is read back from the server, and destroying the resource only removes it from state.

## Example Usage

This example rectifies the zone after the `www` record is created and whenever its record values change. Configure DNSSEC separately in PowerDNS if needed; `powerdns_zone` does not manage DNSSEC settings.

```hcl
resource "powerdns_zone" "example" {
  name = "example.com."
  kind = "Master"
}

resource "powerdns_record" "www" {
  zone    = powerdns_zone.example.name
  name    = "www.example.com."
  type    = "A"
  ttl     = 300
  records = ["192.0.2.1"]
}

resource "powerdns_zone_rectify" "example" {
  zone = powerdns_zone.example.name

  triggers = {
    records = join(",", powerdns_record.www.records)
  }
}
```

## Argument Reference

- `zone` - (Required) The zone to rectify. Changing this runs the operation against the new zone.
- `triggers` - (Optional) A map of arbitrary values. The zone is rectified again whenever any of them change. Without this the operation runs only once.

## Attribute Reference

- `id` - The zone name followed by `:rectify`.
