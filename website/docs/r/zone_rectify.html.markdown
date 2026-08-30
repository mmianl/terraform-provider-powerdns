---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_rectify"
sidebar_current: "docs-powerdns-resource-zone-rectify"
description: |-
  Rectifies a zone, rebuilding its DNSSEC ordering and auth records.
---

# powerdns_zone_rectify

Rectifies a zone, rebuilding its DNSSEC ordering and auth records. This is the same operation as `pdnsutil rectify-zone`, and is mainly useful for zones that are not rectified automatically. See the `api_rectify` argument on `powerdns_zone` for the automatic option.

~> **Note:** This is an operation rather than a tracked object. It runs when the resource is created and whenever `triggers` change, nothing is read back from the server, and destroying the resource only removes it from state.

## Example Usage

```hcl
resource "powerdns_zone" "example" {
  name   = "example.com."
  kind   = "Master"
  dnssec = true
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
