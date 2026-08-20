---
layout: "powerdns"
page_title: "PowerDNS: powerdns_network"
sidebar_current: "docs-powerdns-resource-network"
description: |-
  Manages PowerDNS authoritative network-to-view mappings.
---

# powerdns_network

~> **Server requirements** PowerDNS authoritative views and networks require
the LMDB backend, `views=yes`, and a non-zero `zone-cache-refresh-interval`.
Generic SQL backends do not implement views or networks, so this resource
cannot be used with them.

Manages one PowerDNS authoritative network-to-view mapping.

PowerDNS networks map client source networks to views. Use this resource together with `powerdns_view` to route clients from a CIDR range to a specific view.

## Example Usage

```hcl
resource "powerdns_zone" "internal" {
  name = "internal.example.com."
  kind = "Native"
}

resource "powerdns_view_zone_association" "internal" {
  view = "internal"
  zone = powerdns_zone.internal.name
}

resource "powerdns_network" "internal_clients" {
  network = "192.0.2.0/24"
  view    = powerdns_view_zone_association.internal.view
}
```

## Argument Reference

The following arguments are supported:

- `network` - (Required, Forces new resource) Client source network in CIDR notation, for example `192.0.2.0/24` or `2001:db8::/32`.
- `view` - (Required) Name of the PowerDNS view assigned to the network.

## Importing

Import using the network CIDR.

Example:

```bash
terraform import powerdns_network.internal_clients '192.0.2.0/24'
```
