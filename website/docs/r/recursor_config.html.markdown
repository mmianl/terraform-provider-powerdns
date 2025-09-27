---
layout: "powerdns"
page_title: "PowerDNS: powerdns_recursor_config"
sidebar_current: "docs-powerdns-recursor-config"
description: |-
  Provides a PowerDNS recursor config resource for managing PowerDNS recursor configuration settings.
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

The following arguments are supported:

- `name` - (Required) The name of the recursor configuration setting.
- `value` - (Required) The value of the recursor configuration setting.

## Configuration Types

PowerDNS Recursor configuration settings are categorized by their runtime behavior:

### Configuration Types
- **R/W (Read/Write)**: Can be modified during runtime via rec_control (no restart required)
- **R (Read-only)**: Can only be read, not modified during runtime
- **S (Startup-only)**: Requires restart to change

## Example Configurations

### Network Access Control (R/W)
```hcl
resource "powerdns_recursor_config" "allow_from" {
  name  = "allow-from"
  value = "192.168.0.0/16, 10.0.0.0/8"
}
```

### Cache Configuration (R/W)
```hcl
resource "powerdns_recursor_config" "cache_entries" {
  name  = "max-cache-entries"
  value = "1000000"
}
```

### TTL Override (R/W)
```hcl
resource "powerdns_recursor_config" "ttl_override" {
  name  = "minimum-ttl-override"
  value = "60"
}
```

### Throttling Exceptions (R/W)
```hcl
resource "powerdns_recursor_config" "no_throttle" {
  name  = "dont-throttle-names"
  value = "example.com, critical.com"
}
```

## Configuration Reference

This section provides a comprehensive reference of all PowerDNS Recursor configuration settings organized by category.

### Legend
- **R/W**: Read and Write (can be modified during runtime)
- **R**: Read only (cannot be modified during runtime)
- **S**: Startup only (requires restart to change)

---

## Network & Listening Configuration (Startup Only - S)

| Setting | Description |
|---------|-------------|
| `local-address` | Local IP addresses to bind to |
| `local-port` | Local port to bind to |
| `non-local-bind` | Bind to non-local addresses |
| `reuseport` | Enable SO_REUSEPORT for multiple threads |
| `tcp-fast-open` | Enable TCP Fast Open support |
| `tcp-fast-open-connect` | Enable TCP Fast Open for outgoing connections |

## Access Control & Security (R/W)

| Setting | Description |
|---------|-------------|
| `allow-from` | Netmasks allowed to use the server |
| `allow-from-file` | File containing allowed netmasks |
| `allow-notify-for` | Domains permitted for NOTIFY operations |
| `allow-notify-for-file` | File containing NOTIFY-allowed domains |
| `allow-notify-from` | IPs allowed to send NOTIFY operations |
| `allow-notify-from-file` | File containing NOTIFY-allowed IPs |
| `allow-no-rd` | Allow no recursion desired (RD=0) queries |
| `allow-trust-anchor-query` | Allow trust anchor queries |
| `proxy-protocol-from` | Ranges requiring Proxy Protocol v2 |
| `proxy-protocol-exceptions` | Exceptions to Proxy Protocol requirement |

## Caching & Performance (Mixed Permissions)

| Setting | Type | Description |
|---------|------|-------------|
| `max-cache-entries` | R/W | Maximum DNS record cache entries |
| `max-cache-ttl` | S | Maximum cache TTL |
| `max-cache-bogus-ttl` | S | Maximum bogus record cache TTL |
| `minimum-ttl-override` | R/W | Artificially raise all TTLs |
| `max-negative-ttl` | S | Maximum negative answer cache TTL |
| `max-packetcache-entries` | R/W | Maximum packet cache entries |
| `packetcache-ttl` | S | Maximum packet cache TTL |
| `packetcache-negative-ttl` | S | Maximum negative packet cache TTL |
| `packetcache-servfail-ttl` | S | Maximum ServFail packet cache TTL |
| `packetcache-shards` | S | Number of packet cache shards |
| `disable-packetcache` | S | Disable packet cache |
| `record-cache-locked-ttl-perc` | S | Record cache replacement protection |
| `record-cache-shards` | S | Number of record cache shards |
| `refresh-on-ttl-perc` | S | Refresh percentage for cache entries |
| `serve-stale-extensions` | S | Maximum stale record extensions |

## Forwarding & Zones (R/W)

| Setting | Description |
|---------|-------------|
| `forward-zones` | Zones to forward |
| `forward-zones-file` | File containing forward zones |
| `forward-zones-recurse` | Zones to forward recursively |
| `auth-zones` | Authoritative zones |
| `serve-rfc1918` | Serve RFC1918 reverse zones |
| `serve-rfc6303` | Serve RFC6303 zones |

## Query Processing & Limits (S)

| Setting | Description |
|---------|-------------|
| `max-qperq` | Maximum queries per query resolution |
| `max-cnames-followed` | Maximum CNAME chain length |
| `max-chain-length` | Maximum query chain length |
| `max-concurrent-requests-per-tcp-connection` | TCP concurrent request limit |
| `max-mthreads` | Maximum MTasker threads |
| `max-recursion-depth` | Maximum recursion depth |
| `max-tcp-clients` | Maximum TCP connections |
| `max-tcp-per-client` | Maximum TCP connections per client |
| `max-tcp-queries-per-connection` | Maximum queries per TCP connection |
| `max-total-msec` | Maximum wallclock time per query |
| `max-udp-queries-per-round` | Maximum UDP queries per processing round |
| `max-ns-address-qperq` | Maximum NS address queries per query |
| `max-ns-per-resolve` | Maximum NS records per resolve |
| `limit-qtype-any` | Limit ANY query responses |
| `max-rrset-size` | Maximum RRSet size |
| `network-timeout` | Authoritative server timeout |
| `client-tcp-timeout` | TCP client timeout |

## Network Configuration (S)

| Setting | Description |
|---------|-------------|
| `query-local-address` | Local addresses for outgoing queries |
| `dont-query` | IP addresses to avoid querying |
| `single-socket` | Use single socket for outgoing queries |
| `udp-source-port-min` | Minimum UDP source port |
| `udp-source-port-max` | Maximum UDP source port |
| `udp-source-port-avoid` | UDP ports to avoid |
| `udp-truncation-threshold` | UDP response size limit |

## Carbon/Graphite Integration (R/W)

| Setting | Description |
|---------|-------------|
| `carbon-server` | Carbon server addresses |
| `carbon-interval` | Carbon update interval |
| `carbon-ourname` | Hostname for Carbon metrics |
| `carbon-namespace` | Carbon metric namespace |
| `carbon-instance` | Carbon metric instance name |

## Throttling & Rate Limiting (Mixed)

| Setting | Type | Description |
|---------|------|-------------|
| `dont-throttle-names` | R/W | Names to never throttle |
| `dont-throttle-netmasks` | R/W | Netmasks to never throttle |
| `server-down-max-fails` | S | Server down failure threshold |
| `server-down-throttle-time` | S | Server down throttle duration |
| `bypass-server-throttling-probability` | S | Probability to bypass throttling |
| `non-resolving-ns-max-fails` | S | Non-resolving NS failure threshold |
| `non-resolving-ns-throttle-time` | S | Non-resolving NS throttle time |
| `spoof-nearmiss-max` | S | Spoofing detection threshold |

## Logging & Monitoring (S)

| Setting | Description |
|---------|-------------|
| `loglevel` | Logging verbosity level |
| `log-timestamp` | Include timestamps in logs |
| `log-common-errors` | Log common DNS errors |
| `log-rpz-changes` | Log RPZ zone changes |
| `disable-syslog` | Disable syslog logging |
| `logging-facility` | Syslog facility |
| `quiet` | Suppress query logging |
| `trace` | Enable trace logging |
| `structured-logging` | Enable structured logging |
| `structured-logging-backend` | Structured logging backend |

## Statistics & Metrics (S)

| Setting | Description |
|---------|-------------|
| `statistics-interval` | Statistical summary interval |
| `stats-ringbuffer-entries` | Statistics ringbuffer size |
| `latency-statistic-size` | Query latency averaging size |
| `stats-api-disabled-list` | Disabled API statistics |
| `stats-carbon-disabled-list` | Disabled Carbon statistics |
| `stats-rec-control-disabled-list` | Disabled rec_control statistics |
| `stats-snmp-disabled-list` | Disabled SNMP statistics |

## Web Interface & API (S)

| Setting | Description |
|---------|-------------|
| `webserver` | Enable web interface |
| `webserver-address` | Web interface bind address |
| `webserver-port` | Web interface port |
| `webserver-password` | Web interface password |
| `webserver-allow-from` | Web interface allowed IPs |
| `webserver-hash-plaintext-credentials` | Hash plaintext credentials |
| `webserver-loglevel` | Web interface logging level |
| `api-key` | REST API authentication key |
| `api-config-dir` | API configuration directory |

## Special Features (S)

| Setting | Description |
|---------|-------------|
| `any-to-tcp` | Force ANY queries to TCP |
| `dns64-prefix` | DNS64 IPv6 prefix |
| `lowercase-outgoing` | Lowercase outgoing queries |
| `root-nx-trust` | Trust root NXDOMAIN responses |
| `save-parent-ns-set` | Save parent NS sets |
| `nothing-below-nxdomain` | RFC 8020 NXDOMAIN handling |
| `extended-resolution-errors` | Include extended error codes |

## New Domain Detection (S)

| Setting | Description |
|---------|-------------|
| `new-domain-tracking` | Enable new domain tracking |
| `new-domain-log` | Log newly observed domains |
| `new-domain-lookup` | DNS lookup for new domains |
| `new-domain-db-size` | New domain database size |
| `new-domain-history-dir` | New domain storage directory |
| `new-domain-db-snapshot-interval` | Database snapshot interval |
| `new-domain-ignore-list` | Domains to ignore for new domain detection |
| `new-domain-ignore-list-file` | File with ignored domains |
| `new-domain-pb-tag` | Protobuf tag for new domains |

## Unique Response Tracking (S)

| Setting | Description |
|---------|-------------|
| `unique-response-tracking` | Enable unique response tracking |
| `unique-response-log` | Log unique responses |
| `unique-response-db-size` | Unique response database size |
| `unique-response-history-dir` | Unique response storage directory |
| `unique-response-pb-tag` | Protobuf tag for unique responses |
| `unique-response-ignore-list` | Domains to ignore for unique responses |
| `unique-response-ignore-list-file` | File with ignored domains |

## Configuration & File Management (S)

| Setting | Description |
|---------|-------------|
| `config-dir` | Configuration directory |
| `config-name` | Configuration file prefix |
| `include-dir` | Additional config files directory |
| `ignore-unknown-settings` | Ignore unknown settings |
| `enable-old-settings` | Enable deprecated old-style settings |
| `max-include-depth` | Maximum include file depth |
| `max-generate-steps` | Maximum $GENERATE steps |

## System Integration (S)

| Setting | Description |
|---------|-------------|
| `daemon` | Run as daemon |
| `write-pid` | Write PID file |
| `socket-dir` | Control socket directory |
| `socket-owner` | Control socket owner |
| `socket-group` | Control socket group |
| `socket-mode` | Control socket permissions |
| `system-resolver-ttl` | System resolver TTL |
| `system-resolver-interval` | System resolver check interval |
| `system-resolver-self-resolve-check` | Warn on self-resolve |

## SNMP Integration (S)

| Setting | Description |
|---------|-------------|
| `snmp-agent` | Enable SNMP agent |
| `snmp-daemon-socket` | SNMP daemon socket path |

## Hints & Root Servers (S)

| Setting | Description |
|---------|-------------|
| `hint-file` | Root hints file path |

## Lua Scripting (S)

| Setting | Description |
|---------|-------------|
| `lua-config-file` | Lua configuration file |
| `lua-dns-script` | Lua DNS script file |
| `lua-global-include-dir` | Global Lua include directory |
| `lua-maintenance-interval` | Lua maintenance function interval |

## Miscellaneous (Mixed)

| Setting | Type | Description |
|---------|------|-------------|
| `version-string` | S | Custom version string |
| `server-id` | S | Server identification |
| `security-poll-suffix` | S | Security update check domain |
| `public-suffix-list-file` | S | Public Suffix List file |
| `entropy-source` | S | Random number source (deprecated) |
| `rng` | S | Random number generator (deprecated) |
| `event-trace-enabled` | R/W | Event tracing configuration |
| `protobuf-use-kernel-timestamp` | S | Use kernel timestamps for protobuf |

## Important Notes

- **Provider Configuration**: This resource requires the `recursor_server_url` to be configured in the provider block.
- **Runtime vs Startup**: R/W settings take effect immediately. S (Startup-only) settings require a recursor restart.
- **Read-only Handling**: The provider gracefully handles read-only configurations by using existing values.
- **Error Handling**: If a configuration is read-only, the provider will log a warning and continue with the existing value.

## Troubleshooting

### Testing Configuration Access
Use the diagnostic script to test which configurations are accessible in your environment:

```bash
# Set environment variables
export PDNS_SERVER_URL="https://your-powerdns-server.com"
export PDNS_RECURSOR_SERVER_URL="https://your-recursor-server.com"
export PDNS_API_KEY="your-api-key"

# Run diagnostic script
go run scripts/diagnose-recursor.go
```

### Common Issues

1. **"Read-only" errors**: Some configurations cannot be modified at runtime. This is normal behavior.
2. **Permission errors**: Ensure the API key has sufficient permissions for recursor configuration.
3. **Configuration not found**: Some settings may not exist in older PowerDNS versions.

### Best Practices

1. **Use R/W settings for testing**: Start with known R/W configurations like `allow-from`
2. **Check your PowerDNS version**: Configuration availability varies by version
3. **Plan for restarts**: S (Startup-only) configurations require recursor restarts
4. **Use descriptive names**: Choose configuration names that clearly indicate their purpose

## Related Resources

- [`powerdns_recursor_forward_zone`](recursor_forward_zone.html) - For managing forward zone configurations
