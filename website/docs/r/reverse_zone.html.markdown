---
layout: "powerdns"
page_title: "PowerDNS: powerdns_reverse_zone"
sidebar_current: "docs-powerdns-reverse-zone"
description: |-
  Provides a PowerDNS reverse zone resource for managing in-addr.arpa and ip6.arpa zones. This resource is specifically designed for handling reverse DNS zones for both IPv4 and IPv6 networks.
---

# powerdns_reverse_zone

Provides a PowerDNS reverse zone resource for managing in-addr.arpa and ip6.arpa zones. This resource is specifically designed for handling reverse DNS zones for both IPv4 and IPv6 networks.

## Example Usage

### Using IPv4 CIDR notation

```hcl
resource "powerdns_reverse_zone" "zone_172_16_0_0_24" {
  cidr = "172.16.0.0/24"
  kind = "Master"
  nameservers = [
    "ns01.example.com.",
    "ns02.example.com.",
  ]
}
```

### Using IPv6 CIDR notation

```hcl
resource "powerdns_reverse_zone" "zone_2001_db8" {
  cidr = "2001:db8::/32"
  kind = "Master"
  nameservers = [
    "ns01.example.com.",
    "ns02.example.com.",
  ]
}
```

## Argument Reference

This resource supports the following arguments:

- `cidr` - (Required) The CIDR block for the reverse zone (e.g., '172.16.0.0/16' or '2001:db8::/32'). For IPv4, must have a prefix length of 8, 16, or 24. For IPv6, must have a prefix length that is a multiple of 4 between 4 and 124.
- `kind` - (Required) The kind of zone. Must be either "Master" or "Slave".
- `nameservers` - (Required) List of nameservers for this zone. Each nameserver must be a valid FQDN ending with a dot.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

- `name` - Computed zone name (e.g., '16.172.in-addr.arpa.' for IPv4 or '8.b.d.0.1.0.0.2.ip6.arpa.' for IPv6).

## Notes

- For IPv4 /24 networks, the zone name will include the third octet (e.g., '0.16.172.in-addr.arpa.').
- For IPv6 networks, the zone name will be based on the nibbles (4 bits) of the address in reverse order (e.g., '8.b.d.0.1.0.0.2.ip6.arpa.' for 2001:db8::/32).

## Importing

An existing reverse zone can be imported into this resource by supplying the zone name. If the zone is not found, an error will be returned.

For example, to import zone `16.172.in-addr.arpa.`:

```bash
terraform import powerdns_reverse_zone.test 16.172.in-addr.arpa.
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
