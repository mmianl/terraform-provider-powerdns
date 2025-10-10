---
layout: "powerdns"
page_title: "PowerDNS: powerdns_dnsdist_statistics"
sidebar_current: "docs-powerdns-datasource-dnsdist-statistics"
description: |-
  Retrieves DNSdist statistics and performance metrics.
---

# powerdns_dnsdist_statistics

The `powerdns_dnsdist_statistics` data source allows you to retrieve DNSdist statistics and performance metrics for monitoring and alerting purposes.

## Example Usage

```hcl
# Basic statistics retrieval
data "powerdns_dnsdist_statistics" "example" {
}

# Use statistics in outputs
output "query_count" {
  value = data.powerdns_dnsdist_statistics.example.statistics[0].value
}

output "cache_hit_rate" {
  value = data.powerdns_dnsdist_statistics.example.statistics[1].value
}
```

## Extracting Specific Statistics

You can extract specific statistic values for use in outputs, monitoring, or other resources:

```hcl
# Get total query count (first statistic)
output "total_queries" {
  value = data.powerdns_dnsdist_statistics.example.statistics[0].value
}

# Get cache hit rate using a for expression
output "cache_hits" {
  value = [for stat in data.powerdns_dnsdist_statistics.example.statistics : stat.value if stat.name == "cache-hits"][0]
}

# Create a map of all statistics
output "all_metrics" {
  value = {
    for stat in data.powerdns_dnsdist_statistics.example.statistics :
    stat.name => stat.value
  }
}

# Use in monitoring or alerting
resource "local_file" "dnsdist_metrics" {
  filename = "/tmp/dnsdist-metrics.json"
  content = jsonencode({
    queries = [for stat in data.powerdns_dnsdist_statistics.example.statistics : stat.value if stat.name == "queries"][0]
    cache_hits = [for stat in data.powerdns_dnsdist_statistics.example.statistics : stat.value if stat.name == "cache-hits"][0]
    timestamp = timestamp()
  })
}
```

## Common DNSdist Statistics

The following are some commonly used DNSdist statistics you might want to monitor:

- `queries` - Total number of queries received
- `responses` - Total number of responses sent
- `cache-hits` - Number of cache hits
- `cache-misses` - Number of cache misses
- `latency` - Average response latency
- `servfail-responses` - Number of server failure responses
- `noerror-responses` - Number of successful responses

## Argument Reference

This data source has no required or optional arguments.

## Attributes Reference

The following attributes are exported:

- `statistics` - A list of DNSdist statistics. Each statistic has the following attributes:
  - `name` - The name of the statistic (e.g., "queries", "cache-hits", "cache-misses")
  - `value` - The numeric value of the statistic
  - `type` - The type of the statistic (e.g., "Counter", "Gauge")
  - `tags` - A list of tags associated with the statistic
