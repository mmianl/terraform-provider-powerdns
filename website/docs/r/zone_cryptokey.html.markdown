---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone_cryptokey"
sidebar_current: "docs-powerdns-resource-zone-cryptokey"
description: |-
  Manages DNSSEC keys for a zone on a PowerDNS authoritative server.
---

# powerdns_zone_cryptokey

Manages DNSSEC keys (cryptokeys) for a zone on a PowerDNS authoritative server. PowerDNS can generate the key material, or you can import an existing private key.

~> **Note:** The zone must have DNSSEC enabled for its keys to be used. See the `dnssec` argument on `powerdns_zone`.

## Example Usage

Let PowerDNS generate a combined signing key:

```hcl
resource "powerdns_zone" "example" {
  name   = "example.com."
  kind   = "Native"
  dnssec = true
}

resource "powerdns_zone_cryptokey" "example" {
  zone      = powerdns_zone.example.name
  keytype   = "csk"
  algorithm = "ecdsa256"
}

output "ds_records" {
  value = powerdns_zone_cryptokey.example.ds
}
```

An RSA key needs an explicit size:

```hcl
resource "powerdns_zone_cryptokey" "example_rsa" {
  zone      = powerdns_zone.example.name
  keytype   = "ksk"
  algorithm = "rsasha256"
  bits      = 2048
}
```

A key can be created inactive and enabled later, which is useful for key rollovers:

```hcl
resource "powerdns_zone_cryptokey" "standby" {
  zone      = powerdns_zone.example.name
  keytype   = "zsk"
  algorithm = "ecdsa256"
  active    = false
  published = false
}
```

## Argument Reference

This resource supports the following arguments:

- `zone` - (Required) The zone this key belongs to. Changing this forces a new key.
- `keytype` - (Required) The role of the key: `ksk`, `zsk` or `csk`. Changing this forces a new key.
- `algorithm` - (Optional) The signing algorithm, for example `ecdsa256`, `ecdsa384`, `ed25519`, `rsasha256` or `rsasha512`. Changing this forces a new key.
- `bits` - (Optional) The key size in bits. Required for the RSA algorithms; the server derives it for the ECDSA and Ed curves. Changing this forces a new key.
- `content` - (Optional) An existing private key in ISC format to import. When omitted, PowerDNS generates the key material. Changing this forces a new key.
- `active` - (Optional) Whether the key is used for signing. Defaults to `true`.
- `published` - (Optional) Whether the DNSKEY record is published in the zone. Defaults to `true`.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - The resource ID, in the form `<zone>:<key_id>`.
- `key_id` - The numeric ID PowerDNS assigned to this key.
- `dnskey` - The DNSKEY record for this key.
- `ds` - The DS records for this key, to be published in the parent zone.
- `cds` - The DS records for this key, filtered by the zone's CDS publication settings.

~> **Note:** Some backends store a single combined key and report every key as `csk` regardless of the requested `keytype`.

## Importing

An existing key can be imported using `<zone>:<key_id>`:

```bash
terraform import powerdns_zone_cryptokey.example example.com.:1
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
