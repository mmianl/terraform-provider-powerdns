---
layout: "powerdns"
page_title: "PowerDNS: powerdns_reverse_zone"
sidebar_current: "docs-powerdns-datasource-reverse-zone"
description: |-
  Provides a PowerDNS reverse zone data source for querying in-addr.arpa and ip6.arpa zones.
---

# powerdns_reverse_zone

Provides a PowerDNS reverse zone data source for querying existing in-addr.arpa and ip6.arpa zones. This data source allows you to retrieve information about reverse DNS zones for both IPv4 and IPv6 networks.

## Example Usage

### Querying an IPv4 reverse zone

```hcl
data "powerdns_reverse_zone" "zone_172_16_0_0_16" {
  cidr = "172.16.0.0/16"
}

output "zone_name" {
  value = data.powerdns_reverse_zone.zone_172_16_0_0_16.name
}

output "zone_kind" {
  value = data.powerdns_reverse_zone.zone_172_16_0_0_16.kind
}

output "nameservers" {
  value = data.powerdns_reverse_zone.zone_172_16_0_0_16.nameservers
}

# Query the reverse zone for a /24 network
data "powerdns_reverse_zone" "example_reverse" {
  cidr = "192.168.1.0/24"
}

# Create a PTR record using the dynamically determined zone
resource "powerdns_record" "webserver_ptr" {
  zone    = data.powerdns_reverse_zone.example_reverse.name
  name    = "10.1.168.192.in-addr.arpa."
  type    = "PTR"
  ttl     = 300
  records = ["webserver.example.com."]
}

# Alternative: Create PTR record for a specific IP in the zone
resource "powerdns_record" "mailserver_ptr" {
  zone    = data.powerdns_reverse_zone.example_reverse.name
  name    = "5.1.168.192.in-addr.arpa."
  type    = "PTR"
  ttl     = 300
  records = ["mail.example.com."]
}
```

### Querying an IPv6 reverse zone

```hcl
data "powerdns_reverse_zone" "zone_2001_db8" {
  cidr = "2001:db8::/32"
}

output "zone_name" {
  value = data.powerdns_reverse_zone.zone_2001_db8.name
}

# Query an IPv6 reverse zone
data "powerdns_reverse_zone" "ipv6_reverse" {
  cidr = "2001:db8::/32"
}

# Create PTR record for IPv6 address
resource "powerdns_record" "ipv6_host_ptr" {
  zone    = data.powerdns_reverse_zone.ipv6_reverse.name
  name    = "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa."
  type    = "PTR"
  ttl     = 300
  records = ["ipv6-host.example.com."]
}
```

## Argument Reference

This resource supports the following arguments:

- `cidr` - (Required) The CIDR block for the reverse zone (e.g., '172.16.0.0/16' or '2001:db8::/32'). For IPv4, must have a prefix length of 8, 16, or 24. For IPv6, must have a prefix length that is a multiple of 4 between 4 and 124.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

- `name` - Computed zone name (e.g., '16.172.in-addr.arpa.' for IPv4 or '8.b.d.0.1.0.0.2.ip6.arpa.' for IPv6).
- `kind` -  Kind of zone (Master or Slave).
- `nameservers` - List of nameservers for this zone. Each nameserver is a valid FQDN ending with a dot.

## Notes

- The data source will return an error if the reverse zone for the specified CIDR does not exist in PowerDNS.
- For IPv4 /24 networks, the zone name will include the third octet (e.g., '0.16.172.in-addr.arpa.').
- For IPv6 networks, the zone name will be based on the nibbles (4 bits) of the address in reverse order (e.g., '8.b.d.0.1.0.0.2.ip6.arpa.' for 2001:db8::/32).
