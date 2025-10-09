---
layout: "powerdns"
page_title: "PowerDNS: powerdns_dnsdist_rule"
sidebar_current: "docs-powerdns-dnsdist-rule"
description: |-
  Manages a DNSdist rule for traffic routing and filtering.
---

# powerdns_dnsdist_rule

The `powerdns_dnsdist_rule` resource allows you to manage DNSdist rules for traffic routing, filtering, and load balancing.

## Example Usage

```hcl
# Basic DNSdist rule
resource "powerdns_dnsdist_rule" "example" {
  name        = "block-malicious-domains"
  rule        = "qname == 'malicious.example.com'"
  action      = "Drop"
  enabled     = true
  description = "Block access to known malicious domains"
}

# Rule with pool action
resource "powerdns_dnsdist_rule" "pool_example" {
  name        = "load-balance-backend"
  rule        = "qname == 'api.example.com'"
  action      = "Pool"
  enabled     = true
  description = "Load balance API traffic across backend servers"
}
```

## Argument Reference

The following arguments are supported:

- `name` - (Required) A unique name for the rule.
- `rule` - (Required) The DNSdist rule expression that determines when this rule matches.
- `action` - (Required) The action to take when the rule matches. Common actions include `Drop`, `Pool`, `QPSPool`, `TCP`, `Spoof`, etc.
- `enabled` - (Optional) Whether the rule is enabled. Defaults to `true`.
- `description` - (Optional) A human-readable description of the rule.

## Import

DNSdist rules can be imported using their ID:

```bash
terraform import powerdns_dnsdist_rule.example rule-id
```
