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

Keep the secret out of state with a write-only value (needs Terraform 1.11 or later). Terraform cannot see a write-only value change, so bump `key_wo_version` when you want to send a new secret:

```hcl
resource "powerdns_tsigkey" "transfer" {
  name           = "transfer-key"
  algorithm      = "hmac-sha256"
  key_wo         = var.tsig_secret
  key_wo_version = 1
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
- `key` - (Optional) The base64 encoded secret. When omitted, PowerDNS generates the key material and it is saved to state. Conflicts with `key_wo`.
- `key_wo` - (Optional, Write-only) The base64 encoded secret. It is never saved to the plan or state. Needs Terraform 1.11 or later, and requires `key_wo_version`. Conflicts with `key`.
- `key_wo_version` - (Optional) A number, 1 or more. Change it to send the current `key_wo` value to PowerDNS again, for example to rotate the secret. Requires `key_wo`.

## Attribute Reference

- `id` - The ID of the key, which is the key name followed by a trailing dot (e.g. `transfer-key.`).

~> **Note:** Unless you use `key_wo`, the secret is stored in Terraform state in plain text. Protect your state file accordingly.

## Importing

An existing TSIG key can be imported using its ID:

```bash
terraform import powerdns_tsigkey.transfer transfer-key.
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
