---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_notify"
sidebar_current: "docs-powerdns-resource-zone-notify"
description: |-
  Sends a NOTIFY for a zone to its slaves.
---

# powerdns_zone_notify

Sends a NOTIFY for a zone to its slaves, the same operation as `pdnsutil notify`.

~> **Note:** This is an operation rather than a tracked object. It runs when the resource is created and whenever `triggers` change, nothing is read back from the server, and destroying the resource only removes it from state. A NOTIFY that has already been sent cannot be undone.

## Example Usage

Notify once, after the zone is created:

```hcl
resource "powerdns_zone_notify" "example" {
  zone = powerdns_zone.example.name
}
```

Notify again whenever a record changes:

```hcl
resource "powerdns_zone_notify" "example" {
  zone = powerdns_zone.example.name

  triggers = {
    records = join(",", powerdns_record.www.records)
  }
}
```

## Argument Reference

- `zone` - (Required) The zone to notify for. Changing this runs the operation against the new zone.
- `triggers` - (Optional) A map of arbitrary values. The NOTIFY is sent again whenever any of them change. Without this the operation runs only once.

## Attribute Reference

- `id` - The zone name followed by `:notify`.
