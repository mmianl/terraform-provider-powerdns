---
layout: "powerdns"
page_title: "PowerDNS: powerdns_tsigkey"
sidebar_current: "docs-powerdns-datasource-tsigkey"
description: |-
  Retrieves an existing TSIG key from a PowerDNS authoritative server.
---

# powerdns_tsigkey

Retrieves an existing TSIG key from a PowerDNS authoritative server.

## Example Usage

```hcl
data "powerdns_tsigkey" "transfer" {
  id = "transfer-key."
}

output "algorithm" {
  value = data.powerdns_tsigkey.transfer.algorithm
}
```

## Argument Reference

- `id` - (Required) The ID of the TSIG key to retrieve, which is the key name followed by a trailing dot (e.g. `transfer-key.`).

## Attribute Reference

- `name` - The name of the TSIG key.
- `algorithm` - The TSIG algorithm of the key.
- `key` - The base64 encoded secret of the key.

~> **Note:** The secret is stored in Terraform state in plain text. Protect your state file accordingly.
