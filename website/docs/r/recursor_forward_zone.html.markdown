---
layout: "powerdns"
page_title: "PowerDNS: powerdns_recursor_forward_zone"
sidebar_current: "docs-powerdns-recursor-forward-zone"
description: |-
  Provides a PowerDNS recursor forward zone resource for managing DNS forwarding configuration via the recursor API.
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

This resource supports the following arguments:

- `zone` - (Required) The DNS zone name to forward queries for.
- `servers` - (Required) A list of DNS server IP addresses to forward queries to for this zone.

## How Forward Zones Work

Forward zones in PowerDNS Recursor allow you to specify which DNS servers should handle queries for specific domain names. When a query is received for a domain that matches a forward zone, the recursor will forward the query to the specified servers instead of resolving it through the normal recursive process.

The forward zone configuration is stored in the `forward-zones` setting of the PowerDNS Recursor, which uses the following format:
```
zone1=server1,server2;zone2=server3,server4
```

For example:
```
example.com=192.0.2.1,192.0.2.2;internal.company.com=10.0.0.53
```

## Important Notes

- **Runtime Configuration**: The `forward-zones` setting is **read/write (R/W)**, meaning it can be modified during runtime without requiring a recursor restart.
- **Provider Configuration**: This resource requires the `recursor_server_url` to be configured in the provider block.
- **Multiple Zones**: Multiple forward zones can be configured independently and will be merged together.
- **Immediate Effect**: Changes take effect immediately in the running recursor.
- **Format Requirements**: Zone names should be fully qualified (e.g., `example.com.` with trailing dot, or `example.com` without).
- **Server Requirements**: DNS server addresses should be specified as IP addresses (IPv4 or IPv6).

## Configuration Format Details

Each forward zone entry follows this pattern:
```
<zone_name>=<server1>[,<server2>][,<serverN>]
```

- **zone_name**: The DNS zone to forward (e.g., `example.com`)
- **servers**: Comma-separated list of DNS server IP addresses

Multiple zones are separated by semicolons:
```
zone1=server1;zone2=server2,server3
```

## Troubleshooting

### Common Issues

1. **Forward zones not working**: Ensure the DNS servers are reachable and responding to queries on port 53.

2. **Configuration not taking effect**: Verify that the `recursor_server_url` is correctly configured and the recursor API is accessible.

3. **Permission errors**: Ensure the API key has sufficient permissions to modify recursor configuration.

### Testing Forward Zones

You can test if forward zones are working using the diagnostic script:

```bash
# Set environment variables
export PDNS_SERVER_URL="https://your-powerdns-server.com"
export PDNS_RECURSOR_SERVER_URL="https://your-recursor-server.com"
export PDNS_API_KEY="your-api-key"

# Run diagnostic script
go run scripts/diagnose-recursor.go
```

The script will test forward-zones configuration and other recursor settings.

## Example Configurations

### Basic Forward Zone
```hcl
resource "powerdns_recursor_forward_zone" "example" {
  zone    = "example.com"
  servers = ["192.0.2.1", "192.0.2.2"]
}
```

### Multiple Servers with Fallback
```hcl
resource "powerdns_recursor_forward_zone" "redundant" {
  zone    = "critical.com"
  servers = ["192.0.2.1", "192.0.2.2", "192.0.2.3"]
}
```

### Internal DNS Forwarding
```hcl
resource "powerdns_recursor_forward_zone" "internal" {
  zone    = "internal.company.com"
  servers = ["10.0.0.53"]
}
```

### IPv6 Forward Servers
```hcl
resource "powerdns_recursor_forward_zone" "ipv6_example" {
  zone    = "ipv6.example.com"
  servers = ["2001:db8::1", "2001:db8::2"]
}
```

## Related Resources

- [`powerdns_recursor_config`](recursor_config.html) - For managing other recursor configuration settings
