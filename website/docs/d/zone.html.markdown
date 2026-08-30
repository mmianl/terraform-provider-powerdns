---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone"
sidebar_current: "docs-powerdns-datasource-zone"
description: |-
  Provides a PowerDNS zone data source for querying existing zones with their associated records.
---

# powerdns_zone

Provides a PowerDNS zone data source for querying existing zones. This data source allows you to retrieve information about zones and all their associated DNS records, providing a complete view of zone configuration and record data.

## Example Usage

### Querying a zone with all its records

```hcl
data "powerdns_zone" "example" {
  name = "example.com."
}

output "zone_info" {
  value = {
    name    = data.powerdns_zone.example.name
    kind    = data.powerdns_zone.example.kind
    account = data.powerdns_zone.example.account
    records = data.powerdns_zone.example.records
  }
}

output "a_records" {
  value = [
    for record in data.powerdns_zone.example.records :
    record
    if record.type == "A"
  ]
}

output "mx_records" {
  value = [
    for record in data.powerdns_zone.example.records :
    record
    if record.type == "MX"
  ]
}
```

### Querying a Slave zone

```hcl
data "powerdns_zone" "slave" {
  name = "slave.example.com."
}

output "masters" {
  value = data.powerdns_zone.slave.masters
}
```

## Argument Reference

This resource supports the following arguments:

- `name` - (Required) The fully qualified domain name (FQDN) of the zone to retrieve, ending with a trailing dot (e.g., `"example.com."`).

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

- `kind` - The kind of zone (Master, Slave, etc.).
- `account` - The account associated with the zone (defaults to "admin").
- `masters` - Set of master servers for this zone (Slave zones only).
- `soa_edit_api` - SOA edit API setting.
- `catalog` - The catalog zone this zone belongs to.
- `soa_edit` - SOA edit setting applied when serving the zone.
- `dnssec` - Whether DNSSEC is enabled for this zone.
- `api_rectify` - Whether the zone is rectified automatically after API changes.
- `master_tsig_key_ids` - Set of dotted TSIG key IDs allowed to authenticate AXFR requests for this zone.
- `slave_tsig_key_ids` - Set containing the dotted TSIG key ID used to sign requests to the primary server, if configured.
- `serial` - The current serial of the zone.
- `notified_serial` - The last serial this zone was notified for.
- `edited_serial` - The SOA serial after applying the SOA edit rule.
- `last_check` - Timestamp of the last freshness check (Slave zones only).
- `records` - List of all DNS records in the zone. Each record has the following attributes:
  - `name` - The name of the record.
  - `type` - The type of the record (A, AAAA, CNAME, MX, etc.).
  - `content` - The content of the record.
  - `ttl` - The TTL of the record.
  - `disabled` (bool) Whether the record is disabled.

## Notes

- The data source will return an error if the specified zone does not exist in PowerDNS.
- The `records` attribute provides access to all DNS records within the zone, allowing you to filter and process them as needed.
- For Slave zones, the `masters` attribute will contain the list of master servers.
