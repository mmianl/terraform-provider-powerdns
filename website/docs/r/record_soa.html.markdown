---
layout: "powerdns"
page_title: "PowerDNS: powerdns_record_soa"
sidebar_current: "docs-powerdns-resource-record-soa"
description: |-
  Manages a PowerDNS SOA (Start of Authority) record with individual fields for each SOA component.
---

# powerdns_record_soa

Manages a PowerDNS SOA (Start of Authority) record.

~> **Note:** SOA records cannot be managed with the `powerdns_record` resource. Use this resource instead.

## Example Usage

### Basic SOA record

```hcl
resource "powerdns_zone" "example" {
  name         = "example.com."
  kind         = "Native"
  soa_edit_api = "DEFAULT"
}

resource "powerdns_record_soa" "example" {
  zone    = powerdns_zone.example.name
  name    = powerdns_zone.example.name
  ttl     = 3600
  mname   = "ns1.example.com."
  rname   = "hostmaster.example.com."
  refresh = 10800
  retry   = 3600
  expire  = 604800
  minimum = 3600
}
```

### Ignoring serial changes

When `soa_edit_api` is set on the zone, PowerDNS automatically updates the serial number on every change. Use `ignore_changes` to prevent Terraform from detecting drift on the serial, or omit the serial entirely.

```hcl
resource "powerdns_record_soa" "example" {
  zone    = powerdns_zone.example.name
  name    = powerdns_zone.example.name
  ttl     = 3600
  mname   = "ns1.example.com."
  rname   = "hostmaster.example.com."
  serial  = 0
  refresh = 10800
  retry   = 3600
  expire  = 604800
  minimum = 3600

  lifecycle {
    ignore_changes = [serial]
  }
}
```

## Argument Reference

The following arguments are supported:

- `zone` - (Required, ForceNew) The name of the zone containing this SOA record. Must be a fully qualified domain name (FQDN) ending with a trailing dot (e.g., `"example.com."`).
- `name` - (Required, ForceNew) The name of the SOA record (usually the same as the zone name). Must be a fully qualified domain name (FQDN) ending with a trailing dot.
- `ttl` - (Required) The TTL of the SOA record in seconds.
- `mname` - (Required) The primary nameserver for the zone (MNAME field). Must be a fully qualified domain name (FQDN) ending with a trailing dot (e.g., `"ns1.example.com."`).
- `rname` - (Required) The email address of the person responsible for the zone, in DNS format with a dot instead of `@` (RNAME field). Must be a fully qualified domain name (FQDN) ending with a trailing dot. For example, `hostmaster.example.com.` represents `hostmaster@example.com`.
- `serial` - (Optional) The SOA serial number. If omitted or set to `0`, PowerDNS manages it automatically via the zone's `soa_edit_api` setting. This field is always read back from the server.
- `refresh` - (Required) The time interval (in seconds) before the zone should be refreshed by secondary nameservers.
- `retry` - (Required) The time interval (in seconds) before a failed refresh should be retried.
- `expire` - (Required) The upper limit (in seconds) before the zone is considered no longer authoritative.
- `minimum` - (Required) The negative caching TTL in seconds.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

- `id` - The id of the resource, in the format `name:::SOA`.

## Importing

An existing SOA record can be imported by supplying both the record id and zone name as JSON:

```bash
terraform import powerdns_record_soa.example '{"zone": "example.com.", "id": "example.com.:::SOA"}'
```
