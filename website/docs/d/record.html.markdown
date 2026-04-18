---
layout: "powerdns"
page_title: "PowerDNS: powerdns_record"
sidebar_current: "docs-powerdns-datasource-record"
description: |-
  Provides a PowerDNS record data source for querying an existing RRset, including all records and comments.
---

# powerdns_record

Provides a PowerDNS record data source. Use this data source to retrieve an existing RRset, including all record contents and the `comments` list returned by PowerDNS.

## Example Usage

```hcl
data "powerdns_record" "www" {
  zone = "example.com."
  name = "www.example.com."
  type = "A"
}

output "www_records" {
  value = data.powerdns_record.www.records
}

output "www_comments" {
  value = data.powerdns_record.www.comments
}
```

## Argument Reference

The following arguments are supported:

- `zone` - (Required) The fully qualified domain name (FQDN) of the zone containing the RRset, ending with a trailing dot (for example, `example.com.`).
- `name` - (Required) The fully qualified record name ending with a trailing dot (for example, `www.example.com.`).
- `type` - (Required) The RRset type to read, for example `A`, `AAAA`, `MX`, or `TXT`.

## Attribute Reference

This data source exports the following attributes:

- `id` - The composite RRset identifier in the format `<name>:::<type>`.
- `ttl` - The TTL of the RRset in seconds.
- `disabled` - Whether all records in the RRset are disabled in PowerDNS.
- `records` - The set of record contents in the RRset.
- `comments` - The ordered list of RRset comments stored in PowerDNS.
