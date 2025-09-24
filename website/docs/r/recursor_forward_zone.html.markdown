---
layout: "powerdns"
page_title: "PowerDNS: powerdns_recursor_forward_zone"
sidebar_current: "docs-powerdns-recursor-forward-zone"
description: |-
  Provides a PowerDNS recursor forward zone resource for managing DNS forwarding configuration.
---

# powerdns_recursor_forward_zone

Provides a PowerDNS recursor forward zone resource for managing DNS forwarding configuration via the recursor API.

## Example Usage

```hcl
resource "powerdns_recursor_forward_zone" "example" {
  zone    = "example.com"
  servers = ["192.0.2.1", "192.0.2.2"]
}

resource "powerdns_recursor_forward_zone" "internal" {
  zone    = "internal.company.com"
  servers = ["10.0.0.53"]
}
```

## Argument Reference

The following arguments are supported:

- `zone` - (Required) The DNS zone name to forward queries for.

- `servers` - (Required) A list of DNS server IP addresses to forward queries to for this zone.

## Notes

- This resource requires the `recursor_server_url` to be configured in the provider.
- Forward zone configuration is managed through the `forward-zones` recursor setting.
- Multiple forward zones can be configured independently.
- Changes take effect immediately in the running recursor.
