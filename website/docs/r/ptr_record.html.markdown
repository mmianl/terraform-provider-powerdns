---
layout: "powerdns"
page_title: "PowerDNS: powerdns_ptr_record"
sidebar_current: "docs-powerdns-ptr-record"
description: |-
  Provides a PowerDNS PTR record resource for managing reverse DNS records. This resource is specifically designed for handling PTR records in reverse DNS zones for both IPv4 and IPv6 addresses.
---

# powerdns_ptr_record

Provides a PowerDNS PTR record resource for managing reverse DNS records. This resource is specifically designed for handling PTR records in reverse DNS zones for both IPv4 and IPv6 addresses.

## Example Usage

### IPv4 PTR Record

```hcl
# Create a reverse zone first
resource "powerdns_reverse_zone" "zone_172_16_0_0_24" {
  cidr = "172.16.0.0/24"
  kind = "Master"
  nameservers = [
    "ns01.example.com.",
    "ns02.example.com.",
  ]
}

# Create a PTR record in the reverse zone
resource "powerdns_ptr_record" "example_ipv4" {
  ip_address   = "172.16.0.10"
  hostname     = "host01.example.com."
  ttl          = 30
  reverse_zone = powerdns_reverse_zone.zone_172_16_0_0_24.name
}
```

### Using Data Source for Existing Reverse Zone

```hcl
# Query an existing reverse zone (managed outside Terraform)
data "powerdns_reverse_zone" "existing_zone" {
  cidr = "172.16.0.0/24"
}

# Create a PTR record in the existing reverse zone
resource "powerdns_ptr_record" "example_ipv4_existing" {
  ip_address   = "172.16.0.20"
  hostname     = "host02.example.com."
  ttl          = 30
  reverse_zone = data.powerdns_reverse_zone.existing_zone.name
}
```

### IPv6 PTR Record

```hcl
# Create a reverse zone first
resource "powerdns_reverse_zone" "zone_2001_db8" {
  cidr = "2001:db8::/32"
  kind = "Master"
  nameservers = [
    "ns01.example.com.",
    "ns02.example.com.",
  ]
}

# Create a PTR record in the reverse zone
resource "powerdns_ptr_record" "example_ipv6" {
  ip_address   = "2001:db8::1"
  hostname     = "host01.example.com."
  ttl          = 30
  reverse_zone = powerdns_reverse_zone.zone_2001_db8.name
}
```

## Argument Reference

This resource supports the following arguments:

- `ip_address` - (Required) The IP address for which to create the PTR record. Can be either an IPv4 or IPv6 address.
- `hostname` - (Required) The hostname to which the IP address should point. Must be a valid FQDN ending with a dot.
- `ttl` - (Required) The TTL (Time To Live) of the record in seconds.
- `reverse_zone` - (Required) The name of the reverse zone where the PTR record will be created. This can be the output of a `powerdns_reverse_zone` resource or a `powerdns_reverse_zone` data source.

## Notes

- For IPv4 addresses, the PTR record will be created with the format `X.Y.Z.W.in-addr.arpa.` where X, Y, Z, and W are the octets of the IP address in reverse order.
- For IPv6 addresses, the PTR record will be created with the format `X.Y.Z...ip6.arpa.` where X, Y, Z, etc. are the nibbles (4 bits) of the IP address in reverse order.
- The reverse zone must be appropriate for the IP address type (in-addr.arpa for IPv4, ip6.arpa for IPv6).

## Importing

An existing PTR record can be imported into this resource by supplying the zone name and record name. If the record is not found, an error will be returned.

For example, to import a PTR record for IP 172.16.0.10 in zone `0.16.172.in-addr.arpa.`:

```bash
terraform import powerdns_ptr_record.test '{"id":"10.0.16.172.in-addr.arpa.:::PTR", "zone":"0.16.172.in-addr.arpa."}'
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
