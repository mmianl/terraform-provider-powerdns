---
layout: "powerdns"
page_title: "PowerDNS: powerdns_recursor_config"
sidebar_current: "docs-powerdns-recursor-config"
description: |-
  Provides a PowerDNS recursor config resource for managing PowerDNS recursor configuration settings via the recursor API.
---

# powerdns_recursor_config

Provides a PowerDNS recursor config resource for managing PowerDNS recursor configuration settings via the recursor API.

## Example Usage

```hcl
resource "powerdns_recursor_config" "allow_from" {
  name  = "allow-from"
  value = "192.168.0.0/16, 10.0.0.0/8"
}

resource "powerdns_recursor_config" "client_tcp_timeout" {
  name  = "client-tcp-timeout"
  value = "5"
}
```

## Argument Reference

This resource supports the following arguments:

- `name` - (Required) The name of the recursor configuration setting.
- `value` - (Required) The value of the recursor configuration setting.

## Notes

- This resource requires the `recursor_server_url` to be configured in the provider.
- Configuration changes are applied immediately to the running recursor.
- Some configuration settings may require a recursor restart to take effect.
