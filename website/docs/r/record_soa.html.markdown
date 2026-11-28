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
  disabled = false
  mname   = "ns1.example.com."
  rname   = "hostmaster.example.com."
  refresh = 10800
  retry   = 3600
  expire  = 604800
  minimum = 3600
}
```

### Letting PowerDNS manage the serial

When `soa_edit_api` is set on the zone, PowerDNS updates the serial number on every change. Leave `serial` out, or set it to `0`, and read the number PowerDNS holds from `server_serial`.

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
}

output "soa_serial" {
  value = powerdns_record_soa.example.server_serial
}
```

## How the serial behaves

| `serial` in the configuration | What is sent to PowerDNS | `serial` in state | `server_serial` |
| --- | --- | --- | --- |
| omitted | on create `0`, on update the serial PowerDNS holds | the serial PowerDNS reports, which changes when PowerDNS advances it | the serial PowerDNS reports |
| `0` | on create `0`, on update the serial PowerDNS holds | `0`, never compared with the server | the serial PowerDNS reports |
| a number other than `0` | that number, as written | that number | the serial PowerDNS reports |

- `0` means "PowerDNS manages the serial". Terraform does not show a change when PowerDNS advances it. Use `server_serial` to see the real number, for example in outputs or in the `triggers_replace` of a `terraform_data` resource.
- An update never sends `0` or an old serial when `serial` is omitted or `0`. It sends the serial PowerDNS holds, so an update cannot move the serial backwards.
- A number other than `0` is sent as written. The zone's `soa_edit_api` setting can still change the serial when the record is edited, and then `serial` and `server_serial` differ. Use `0` or omit it when `soa_edit_api` should decide.
- `server_serial` is read on every refresh and stays unknown during an update that may advance the serial, so the value is never promised in advance.

## Argument Reference

The following arguments are supported:

- `zone` - (Required, ForceNew) The name of the zone containing this SOA record. Must be a fully qualified domain name (FQDN) ending with a trailing dot (e.g., `"example.com."`).
- `name` - (Required, ForceNew) The name of the SOA record (usually the same as the zone name). Must be a fully qualified domain name (FQDN) ending with a trailing dot.
- `ttl` - (Required) The TTL of the SOA record in seconds.
- `disabled` - (Optional) Whether the SOA record is disabled in PowerDNS. Defaults to `false`.
- `mname` - (Required) The primary nameserver for the zone (MNAME field). Must be a fully qualified domain name (FQDN) ending with a trailing dot (e.g., `"ns1.example.com."`).
- `rname` - (Required) The email address of the person responsible for the zone, in DNS format with a dot instead of `@` (RNAME field). Must be a fully qualified domain name (FQDN) ending with a trailing dot. For example, `hostmaster.example.com.` represents `hostmaster@example.com`.
- `serial` - (Optional) The SOA serial number. If omitted or set to `0`, PowerDNS manages it automatically via the zone's `soa_edit_api` setting. See [How the serial behaves](#how-the-serial-behaves). With `0` the value in state stays `0`; read the actual number from `server_serial`.
- `refresh` - (Required) The time interval (in seconds) before the zone should be refreshed by secondary nameservers.
- `retry` - (Required) The time interval (in seconds) before a failed refresh should be retried.
- `expire` - (Required) The upper limit (in seconds) before the zone is considered no longer authoritative.
- `minimum` - (Required) The negative caching TTL in seconds.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

- `id` - The id of the resource, in the format `name:::SOA`.
- `server_serial` - The serial PowerDNS reports for the SOA record, whatever `serial` is set to.

## Upgrading from earlier versions

The previous version stored the serial PowerDNS reported, even when `serial = 0`. The current version keeps the `0` you configured, so `serial` no longer shows the real number. References such as `powerdns_record_soa.example.serial` that expect the real number should use `powerdns_record_soa.example.server_serial` instead:

```hcl
output "soa_serial" {
  value = powerdns_record_soa.example.server_serial # was: powerdns_record_soa.example.serial
}
```

The first plan after the upgrade can show `serial` changing from the stored number to `0` for a record configured with `serial = 0`. The update sends the serial PowerDNS holds, so the serial on the server does not change. `server_serial` is filled in by the first refresh.

## Importing

An existing SOA record can be imported by supplying both the record id and zone name as JSON:

```bash
terraform import powerdns_record_soa.example '{"zone": "example.com.", "id": "example.com.:::SOA"}'
```
