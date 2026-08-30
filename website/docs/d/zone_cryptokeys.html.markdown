---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_cryptokeys"
sidebar_current: "docs-powerdns-datasource-zone-cryptokeys"
description: |-
  Retrieves all DNSSEC keys configured for a zone on a PowerDNS authoritative server.
---

# powerdns_zone_cryptokeys

Retrieves all DNSSEC keys (cryptokeys) configured for a zone on a PowerDNS authoritative server.

## Example Usage

```hcl
data "powerdns_zone_cryptokeys" "example" {
  zone = "example.com."
}

output "ds_records" {
  value = flatten(data.powerdns_zone_cryptokeys.example.cryptokeys[*].ds)
}
```

## Argument Reference

- `zone` - (Required) The zone whose DNSSEC keys should be retrieved.

## Attribute Reference

- `cryptokeys` - A list of all DNSSEC keys for the zone. Each entry has the following attributes:
  - `key_id` - The numeric ID PowerDNS assigned to this key.
  - `keytype` - The role of the key: `ksk`, `zsk` or `csk`.
  - `active` - Whether the key is used for signing.
  - `published` - Whether the DNSKEY record is published in the zone.
  - `algorithm` - The signing algorithm of the key.
  - `bits` - The key size in bits.
  - `dnskey` - The DNSKEY record for this key.
  - `ds` - The DS records for this key.
  - `cds` - The DS records for this key, filtered by CDS publication settings.
