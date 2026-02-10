---
layout: "powerdns"
page_title: "PowerDNS: powerdns_record_soa"
sidebar_current: "docs-powerdns-datasource-record-soa"
description: |-
  Provides a PowerDNS SOA record data source for querying SOA record details of an existing zone.
---

# powerdns_record_soa

Provides a PowerDNS SOA record data source. Use this data source to retrieve the individual SOA fields (mname, rname, serial, refresh, retry, expire, minimum) for an existing zone.

## Example Usage

```hcl
data "powerdns_record_soa" "example" {
  zone = "example.com."
  name = "example.com."
}

output "soa_serial" {
  value = data.powerdns_record_soa.example.serial
}

output "soa_primary_ns" {
  value = data.powerdns_record_soa.example.mname
}
```

## Argument Reference

The following arguments are supported:

- `zone` - (Required) The fully qualified domain name (FQDN) of the zone containing the SOA record, ending with a trailing dot (for example, `example.com.`).
- `name` - (Required) The fully qualified domain name (FQDN) of the SOA record (usually the same as the zone name), ending with a trailing dot (for example, `example.com.`).

## Attribute Reference

This data source exports the following attributes:

- `ttl` - The TTL of the SOA record in seconds.
- `mname` - The primary nameserver for the zone (MNAME field).
- `rname` - The responsible person email in DNS format (RNAME field).
- `serial` - The SOA serial number.
- `refresh` - The refresh interval in seconds.
- `retry` - The retry interval in seconds.
- `expire` - The expire time in seconds.
- `minimum` - The minimum TTL (negative caching) in seconds.
