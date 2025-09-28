---
layout: "powerdns"
page_title: "PowerDNS: powerdns_record"
sidebar_current: "docs-powerdns-resource-record"
description: |-
  Manages PowerDNS DNS records within Terraform. This resource supports all standard DNS record types including A, AAAA, CNAME, MX, TXT, SRV, and more, with full control over TTL values and record data.
---

# powerdns_record

Manages PowerDNS DNS records within Terraform. This resource supports all standard DNS record types including A, AAAA, CNAME, MX, TXT, SRV, and more, with full control over TTL values and record data.

## Supported Record Types

The `powerdns_record` resource supports all standard DNS record types that PowerDNS supports. Below is a comprehensive list organized by category:

### Basic Address Records

- **A** - IPv4 address records
- **AAAA** - IPv6 address records

### Name Resolution Records

- **CNAME** - Canonical name records (alias records)
- **ALIAS** - Alias records (PowerDNS-specific, similar to CNAME but works at zone apex)

### Mail and Service Records

- **MX** - Mail exchange records
- **SRV** - Service locator records
- **NAPTR** - Naming authority pointer records

### Text and Metadata Records

- **TXT** - Text records (SPF, DKIM, DMARC, etc.)
- **SPF** - Sender Policy Framework records (deprecated, use TXT instead)

### Administrative Records

- **NS** - Name server records
- **SOA** - Start of authority records
- **HINFO** - Host information records
- **LOC** - Location records
- **SSHFP** - SSH public key fingerprint records

### Reverse DNS Records

- **PTR** - Pointer records (typically managed via `powerdns_ptr_record` resource)

## Example Usage

Note that PowerDNS may internally lowercase certain records (e.g. CNAME and AAAA), which may lead to resources being marked for a change in every single plan/apply.

### Record Type Examples

#### A record example

For the v1 API (PowerDNS version 4):

```hcl
# Add A record to the zone
resource "powerdns_record" "foobar" {
  zone    = "example.com."
  name    = "www.example.com."
  type    = "A"
  ttl     = 300
  records = ["192.168.0.11"]
}
```

#### AAAA (IPv6) Records

```hcl
# IPv6 address record
resource "powerdns_record" "ipv6_example" {
  zone    = "example.com."
  name    = "ipv6.example.com."
  type    = "AAAA"
  ttl     = 300
  records = ["2001:db8::1", "2001:db8::2"]
}
```

#### PTR record example

An example creating PTR record:

```hcl
# Add PTR record to the zone
resource "powerdns_record" "foobar" {
  zone    = "0.10.in-addr.arpa."
  name    = "10.0.0.10.in-addr.arpa."
  type    = "PTR"
  ttl     = 300
  records = ["www.example.com."]
}
```

#### MX record example

The following example shows, how to setup MX record with a priority of `10`.
Please note that priority is not set as other `powerdns_record` properties; rather, it's part of the string that goes into `records` list.

```hcl
# Add MX record to the zone with priority 10
resource "powerdns_record" "foobar" {
  zone    = "example.com."
  name    = "example.com."
  type    = "MX"
  ttl     = 300
  records = ["10 mail1.example.com"]
}
```

#### CNAME Records

```hcl
# Canonical name record (alias)
resource "powerdns_record" "cname_example" {
  zone    = "example.com."
  name    = "alias.example.com."
  type    = "CNAME"
  ttl     = 300
  records = ["target.example.com."]
}
```

#### SRV Records

```hcl
# Service locator record for SIP service
resource "powerdns_record" "sip_srv" {
  zone    = "example.com."
  name    = "_sip._tcp.example.com."
  type    = "SRV"
  ttl     = 300
  records = [
    "10 60 5060 sip1.example.com.",
    "20 60 5060 sip2.example.com."
  ]
}
```

#### TXT Records

```hcl
# Text record for SPF
resource "powerdns_record" "spf_txt" {
  zone    = "example.com."
  name    = "example.com."
  type    = "TXT"
  ttl     = 300
  records = ["\"v=spf1 mx -all\""]
}

# Text record for DKIM
resource "powerdns_record" "dkim_txt" {
  zone    = "example.com."
  name    = "default._domainkey.example.com."
  type    = "TXT"
  ttl     = 300
  records = ["\"v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC...\""]
}
```

#### NS Records

```hcl
# Name server delegation
resource "powerdns_record" "ns_delegation" {
  zone    = "example.com."
  name    = "subdomain.example.com."
  type    = "NS"
  ttl     = 300
  records = [
    "ns1.example.com.",
    "ns2.example.com."
  ]
}
```

#### SSHFP Records

```hcl
# SSH public key fingerprint
resource "powerdns_record" "sshfp_example" {
  zone    = "example.com."
  name    = "ssh.example.com."
  type    = "SSHFP"
  ttl     = 300
  records = ["1 1 123456789abcdef67890123456789abcdef67890"]
}
```

#### HINFO Records

```hcl
# Host information
resource "powerdns_record" "hinfo_example" {
  zone    = "example.com."
  name    = "server.example.com."
  type    = "HINFO"
  ttl     = 300
  records = ["\"PC-Intel-2.4ghz\" \"Linux\""]
}
```

#### LOC Records

```hcl
# Location information
resource "powerdns_record" "loc_example" {
  zone    = "example.com."
  name    = "building.example.com."
  type    = "LOC"
  ttl     = 300
  records = ["51 56 0.123 N 5 54 0.000 E 4.00m 1.00m 10000.00m 10.00m"]
}
```

#### ALIAS Records (PowerDNS-specific)

```hcl
# Alias record (works at zone apex)
resource "powerdns_record" "alias_example" {
  zone    = "example.com."
  name    = "example.com."  # Can be at zone apex
  type    = "ALIAS"
  ttl     = 300
  records = ["target.example.com."]
}
```

### Multiple Values for Records

Sometimes you need multiple values for the same DNS resource record, such as multiple IP addresses for load balancing or multiple mail servers.

#### A Records with Multiple IPs

```hcl
# Add multiple A records for load balancing
resource "powerdns_record" "load_balanced" {
  zone    = "example.com."
  name    = "www.example.com."
  type    = "A"
  ttl     = 300
  records = ["192.168.0.11", "192.168.0.12", "192.168.0.13"]
}
```

#### TXT Records with Multiple Values

```hcl
# Multiple TXT records for SPF and DKIM
resource "powerdns_record" "multi_txt" {
  zone    = "example.com."
  name    = "example.com."
  type    = "TXT"
  ttl     = 300
  records = [
    "\"v=spf1 mx -all\"",
    "\"v=DKIM1; k=rsa; s=email; p=Msdsdfsdfsdfsdfsdfsdfsdfsdfsdfsfdfsdfsdfsdfds\""
  ]
}
```

#### MX Records with Multiple Mail Servers

```hcl
# Multiple MX records with different priorities
resource "powerdns_record" "multi_mx" {
  zone    = "example.com."
  name    = "example.com."
  type    = "MX"
  ttl     = 300
  records = [
    "10 mail1.example.com",
    "20 mail2.example.com",
    "30 mail3.example.com"
  ]
}
```

### Automatically set PTR record for A/AAAA records

!> **Deprecation warning:** _set_ptr_ feature is set to be deprecated in PowerDNS v4.3.0

PowerDNS API v4.2.0 offers a feature to automatically create corresponding PTR record for the A/AAAA record.
Existing PTR records with the same name are replaced. If no matching reverse zone is found, resource creation will fail.
You can use `powerdns_zone` resource to create the reverse zone.


!> **Warning:** Using _set_ptr:true_  will not automatically remove the PTR record when A/AAAA record is deleted. You should create PTR zone using `powerdns_zone` and manage PTR records using `powerdns_record`, rather than using _set_ptr_. With upcoming _set_ptr_ deprecation, this will be the only way of maintaining PTR records **via this provider**.

Here is an example of creating A record along with corresponding PTR record:

```hcl
resource "powerdns_record" "foobar" {
  zone    = "example.com."
  name    = "www.example.com"
  type    = "A"
  ttl     = 300
  set_ptr = true
  records = ["192.168.0.11"]
}
```

For the legacy API (PowerDNS version 3.4):

```hcl
# Add a record to the zone
resource "powerdns_record" "foobar" {
  zone    = "example.com."
  name    = "www.example.com."
  type    = "A"
  ttl     = 300
  records = ["192.168.0.11"]
}
```

## Record-Specific Formatting Guidelines

Different DNS record types require specific formatting for their content. Here are the most important formatting rules:

### General Rules

- **FQDN endings**: Most records that reference other domain names should end with a dot (`.`) to indicate they are fully qualified domain names
- **Quoting**: Text values in TXT records must be quoted with double quotes
- **Priority values**: MX and SRV records include priority/weight values as part of their content

#### Record Type Specific Formatting

| Record Type | Format                                                 | Example                                                   |
| ----------- | ------------------------------------------------------ | --------------------------------------------------------- |
| **A**       | IPv4 address                                           | `192.168.1.1`                                             |
| **AAAA**    | IPv6 address                                           | `2001:db8::1`                                             |
| **CNAME**   | Target domain (with trailing dot)                      | `target.example.com.`                                     |
| **MX**      | Priority and mail server                               | `10 mail.example.com.`                                    |
| **SRV**     | Priority, weight, port, target                         | `10 60 5060 sip.example.com.`                             |
| **TXT**     | Quoted text                                            | `"v=spf1 mx -all"`                                        |
| **NS**      | Name server (with trailing dot)                        | `ns1.example.com.`                                        |
| **PTR**     | Target domain (with trailing dot)                      | `host.example.com.`                                       |
| **HINFO**   | Quoted hardware and software                           | `"PC-Intel-2.4ghz" "Linux"`                               |
| **LOC**     | Location coordinates                                   | `51 56 0.123 N 5 54 0.000 E 4.00m 1.00m 10000.00m 10.00m` |
| **SSHFP**   | Algorithm, type, fingerprint                           | `1 1 123456789abcdef67890123456789abcdef67890`            |
| **NAPTR**   | Order, preference, flags, service, regexp, replacement | `100 50 "s" "z3950+I2L+I2C" "" _z3950._tcp.gatech.edu.`   |

#### Common Formatting Mistakes to Avoid

1. **Missing trailing dots**: `CNAME target.example.com` should be `CNAME target.example.com.`
2. **Unquoted TXT values**: `TXT v=spf1 mx -all` should be `TXT "v=spf1 mx -all"`
3. **Missing priority in MX**: `MX mail.example.com.` should be `MX 10 mail.example.com.`
4. **Incorrect SRV format**: `SRV sip.example.com. 5060` should be `SRV 10 60 5060 sip.example.com.`

## Best Practices and Recommendations

### TTL Recommendations by Record Type

| Record Type | Recommended TTL | Notes                                                      |
| ----------- | --------------- | ---------------------------------------------------------- |
| **A/AAAA**  | 300-3600        | Address records, balance between freshness and performance |
| **CNAME**   | 300-3600        | Alias records, should match target record TTL              |
| **MX**      | 300-3600        | Mail routing, important for email delivery                 |
| **TXT**     | 300-86400       | Text records, often cached by email systems                |
| **NS**      | 86400+          | Name server records, should be stable                      |
| **SOA**     | 86400+          | Zone metadata, rarely changes                              |
| **SRV**     | 300-3600        | Service records, balance between performance and updates   |

#### Performance Considerations

- **Use appropriate TTLs**: Lower TTLs provide faster updates but increase DNS query load
- **Group related records**: Consider using the same TTL for records that change together
- **Consider caching**: Email systems and CDNs may cache TXT records longer than specified TTL

#### Security Considerations

- **SPF records**: Use TXT records instead of deprecated SPF type
- **DKIM/DMARC**: Store cryptographic keys in TXT records with appropriate TTL
- **SSHFP**: Consider security implications of publishing SSH fingerprints publicly

#### Troubleshooting Common Issues

1. **Record not resolving**: Check for missing trailing dots in FQDN references
2. **MX records not working**: Verify priority values are included and mail server names are correct
3. **TXT records not validating**: Ensure proper quoting and escaping of special characters
4. **SRV records not found**: Verify the service name format (`_service._protocol.domain`)

## Argument Reference

The following arguments are supported:

- `zone` - (Required) The name of zone to contain this record.
- `name` - (Required) The name of the record.
- `type` - (Required) The record type.
- `ttl` - (Required) The TTL of the record.
- `records` - (Required) A string list of records.
- `set_ptr` (Optional) [**_Deprecated in PowerDNS 4.3.0_**] A boolean (true/false), determining whether API server should automatically create PTR record in the matching reverse zone. Existing PTR records are replaced. If no matching reverse zone, an error is thrown.

### Attribute Reference

This resource exports the following attributes in addition to the arguments above:

- `id` - The id of the resource is a composite of the record name and record type, joined by a separator - `:::`.

For example, record `foo.test.com.` of type `A` will be represented with the following `id`: `foo.test.com.:::A`

### Importing

An existing record can be imported into this resource by supplying both the record id and zone name it belongs to.
If the record or zone is not found, or if the record is of a different type or in a different zone, an error will be returned.

For example:

```bash
terraform import powerdns_record.test-a '{"zone": "test.com.", "id": "foo.test.com.:::A"}'
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
