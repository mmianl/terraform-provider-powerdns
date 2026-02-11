---
layout: "powerdns"
page_title: "PowerDNS: powerdns_zone"
sidebar_current: "docs-powerdns-zone"
description: |-
  Manages DNS zones within a PowerDNS authoritative server. This resource supports creating, updating, and deleting zones with various configuration options including different zone types (Native, Master, Slave) and SOA record customization.
---

# powerdns_zone

Manages DNS zones within a PowerDNS authoritative server. This resource supports creating, updating, and deleting zones with various configuration options including different zone types (Native, Master, Slave) and SOA record customization.

## Migrating from v1 to v2

In v1 of this provider, the `powerdns_zone` resource had a `nameservers` argument that automatically created NS records for the zone. In v2, this argument has been removed. Nameservers are now managed using a separate `powerdns_record` resource with type `"NS"`.

~> **Note:** Removing the `nameservers` argument does **not** delete the NS records from PowerDNS. The records will remain on the server but become unmanaged by Terraform. If you want Terraform to continue managing them, add a `powerdns_record` resource as shown below.

### Before (v1)

```hcl
resource "powerdns_zone" "example" {
  name        = "example.com."
  kind        = "Native"
  nameservers = ["ns1.example.com.", "ns2.example.com."]
}
```

### After (v2)

```hcl
resource "powerdns_zone" "example" {
  name = "example.com."
  kind = "Native"
}

resource "powerdns_record" "example_ns" {
  zone = powerdns_zone.example.name
  name = powerdns_zone.example.name
  type = "NS"
  ttl  = 3600
  records = [
    "ns1.example.com.",
    "ns2.example.com.",
  ]
}
```

No `terraform import` is needed for the new `powerdns_record` NS resource. Since the NS records already exist on the server with the same values, Terraform will adopt them on the first apply.

## Example Usage

For the v1 API (PowerDNS version 4):

```hcl
# Add a zone
resource "powerdns_zone" "foobar" {
  name = "example.com."
  kind = "Native"
}

# Manage nameservers using a powerdns_record resource
resource "powerdns_record" "foobar_ns" {
  zone    = powerdns_zone.foobar.name
  name    = powerdns_zone.foobar.name
  type    = "NS"
  ttl     = 3600
  records = ["ns1.example.com.", "ns2.example.com."]
}
```

```hcl
# Add a Slave zone with list of IPs configured as a master for this zone
resource "powerdns_zone" "fubar" {
  name     = "slave.example.com."
  kind     = "Slave"
  masters  = ["10.10.10.10", "20.20.20.21"]
}
```

## Argument Reference

This resource supports the following arguments:

- `name` - (Required) The name of zone. Must be a fully qualified domain name (FQDN) ending with a trailing dot (e.g., `"example.com."`).
- `kind` - (Required) The kind of the zone.
- `account` - (Optional) The account owning the zone. (Default to "admin")
- `masters` - (Optional) List of IP addresses configured as a master for this zone. This argument must be provided when `kind` is set to `Slave`.
- `soa_edit_api` - (Optional) This should map to one of the [supported API values](https://doc.powerdns.com/authoritative/dnsupdate.html#soa-edit-dnsupdate-settings) *or* in [case you wish to remove the setting](https://doc.powerdns.com/authoritative/domainmetadata.html#soa-edit-api), set this argument as `""` (that will translate to the API value `""`).

## Importing

An existing zone can be imported into this resource by supplying the zone name. If the zone is not found, an error will be returned.

For example, to import zone `test.com.`:

```bash
terraform import powerdns_zone.test test.com.
```

For more information on how to use terraform's `import` command, please refer to terraform's [core documentation](https://www.terraform.io/docs/import/index.html#currently-state-only).
