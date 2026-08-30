---
layout: "powerdns"
page_title: "PowerDNS: powerdns_tsigkey"
sidebar_current: "docs-powerdns-resource-tsigkey"
description: |-
  Manages TSIG keys on a PowerDNS authoritative server. TSIG keys authenticate NOTIFY, AXFR and DNS UPDATE messages between servers.
---

# powerdns_tsigkey

Manages TSIG keys on a PowerDNS authoritative server. TSIG keys authenticate NOTIFY, AXFR and DNS UPDATE messages between servers.

## Example Usage

Let PowerDNS generate the secret:

```hcl
resource "powerdns_tsigkey" "transfer" {
  name      = "transfer-key"
  algorithm = "hmac-sha256"
}
```

Supply your own base64 encoded secret:

```hcl
resource "powerdns_tsigkey" "transfer" {
  name      = "transfer-key"
  algorithm = "hmac-sha256"
  key       = var.tsig_secret
}
```

The key can then be referenced from a zone:

```hcl
resource "powerdns_zone" "example" {
  name                 = "example.com."
  kind                 = "Master"
  master_tsig_key_ids  = [powerdns_tsigkey.transfer.id]
}
```

## Argument Reference

This resource supports the following arguments:

- `name` - (Required) The name of the TSIG key.
- `algorithm` - (Required) The TSIG algorithm. One of `hmac-md5`, `hmac-sha1`, `hmac-sha224`, `hmac-sha256`, `hmac-sha384` or `hmac-sha512`. Changing this forces a new key to be created, because PowerDNS would otherwise keep the previous algorithm alongside the new one under the same name.
- `key` - (Optional) The base64 encoded secret. When omitted, PowerDNS generates the key material and it is saved to state.

## Attribute Reference

- `id` - The ID of the key, which is the key name followed by a trailing dot (e.g. `transfer-key.`).

~> **Note:** The secret is stored in Terraform state in plain text. Protect your state file accordingly.

## Importing

An existing TSIG key can be imported using its ID:

```bash
terraform import powerdns_tsigkey.transfer transfer-key.
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
