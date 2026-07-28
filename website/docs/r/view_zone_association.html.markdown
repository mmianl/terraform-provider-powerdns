---
layout: "powerdns"
page_title: "PowerDNS: powerdns_view_zone_association"
sidebar_current: "docs-powerdns-resource-view-zone-association"
description: |-
  Manages one PowerDNS authoritative view-to-zone association.
---

# powerdns_view_zone_association

~> **Backend requirement** Views and networks are only implemented by the LMDB
backend. On a generic SQL backend a read returns an empty list while a write
fails with `422`, so this resource cannot be used there. Check the `launch=`
setting on the server.

Manages one PowerDNS authoritative view-to-zone association.

Use this resource together with `powerdns_view` to manage each zone membership independently. This is useful when multiple Terraform resources need to add or remove zones from the same view without replacing the whole view membership set.

## Example Usage

```hcl
resource "powerdns_zone" "private" {
  name = "private.example."
  kind = "Native"
}

resource "powerdns_view_zone_association" "private" {
  view = "internal"
  zone = powerdns_zone.private.name
}
```

## Argument Reference

- `view` - (Required, Forces new resource) Name of the PowerDNS view.
- `zone` - (Required, Forces new resource) Name of the zone to associate with the view. This may be a normal FQDN zone such as `example.com.` or a variant zone such as `example.com..internal`.

## Importing

Import using the view name, three colons (`:::`), and the zone name.

```bash
terraform import powerdns_view_zone_association.private 'internal:::private.example.'
```
