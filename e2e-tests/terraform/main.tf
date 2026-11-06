resource "powerdns_zone" "test" {
  name         = "test.example.com."
  kind         = "Native"
  soa_edit_api = "DEFAULT"
}

resource "powerdns_zone_metadata" "test_also_notify" {
  zone     = powerdns_zone.test.name
  kind     = "ALSO-NOTIFY"
  metadata = ["192.0.2.10", "192.0.2.11:5300"]
}

resource "powerdns_zone_metadata" "test_allow_axfr_from" {
  zone     = powerdns_zone.test.name
  kind     = "ALLOW-AXFR-FROM"
  metadata = ["AUTO-NS", "2001:db8::/48"]
}

data "powerdns_zone_metadata" "test_also_notify" {
  zone = powerdns_zone.test.name
  kind = "ALSO-NOTIFY"

  depends_on = [powerdns_zone_metadata.test_also_notify]
}

data "powerdns_zone_metadata_list" "test" {
  zone = powerdns_zone.test.name

  depends_on = [powerdns_zone_metadata.test_also_notify, powerdns_zone_metadata.test_allow_axfr_from]
}

resource "powerdns_record" "test_ns" {
  zone = powerdns_zone.test.name
  name = powerdns_zone.test.name
  type = "NS"
  ttl  = 3600
  records = [
    "ns1.example.com.",
    "ns2.example.com.",
  ]
}

resource "powerdns_zone" "test_slave" {
  name    = "test-slave.example.com."
  kind    = "Slave"
  masters = ["192.168.0.1", "192.168.0.3"]
}

data "powerdns_zone" "test" {
  name = "test.example.com."

  depends_on = [powerdns_zone.test, powerdns_record.host01, powerdns_record_soa.soa, powerdns_record.test_ns]
}

resource "powerdns_reverse_zone" "zone_172_16_0_0_24" {
  cidr = "172.16.0.0/24"
  kind = "Master"
  nameservers = [
    "ns1.example.com.",
    "ns2.example.com.",
  ]
}

data "powerdns_reverse_zone" "zone_172_16_0_0_24" {
  cidr = "172.16.0.0/24"

  depends_on = [powerdns_reverse_zone.zone_172_16_0_0_24]
}

resource "powerdns_record" "host01" {
  zone = powerdns_zone.test.name
  name = "host01.test.example.com."
  type = "A"
  ttl  = 30
  comments = [
    "managed-by=terraform",
    "owner=dns-team",
  ]
  records = [cidrhost("172.16.0.0/24", 10)]
}

resource "powerdns_record" "host02_disabled" {
  zone     = powerdns_zone.test.name
  name     = "host02-disabled.test.example.com."
  type     = "A"
  ttl      = 45
  disabled = true
  comments = [
    "managed-by=terraform",
    "owner=dns-team",
  ]
  records = [
    cidrhost("172.16.0.0/24", 11),
    cidrhost("172.16.0.0/24", 12),
  ]
}

data "powerdns_record" "host02_disabled" {
  zone = powerdns_zone.test.name
  name = powerdns_record.host02_disabled.name
  type = powerdns_record.host02_disabled.type

  depends_on = [powerdns_record.host02_disabled]
}

resource "powerdns_ptr_record" "host01_ipv4" {
  ip_address   = "172.16.0.10"
  hostname     = "host01.test.example.com."
  ttl          = 30
  reverse_zone = powerdns_reverse_zone.zone_172_16_0_0_24.name
}

# https://doc.powerdns.com/recursor/http-api/endpoint-servers-config.html
resource "powerdns_recursor_config" "allow_from" {
  name  = "allow-from"
  value = ["192.168.0.0/16", "10.0.0.0/8"]
}

resource "powerdns_recursor_config" "allow_notify_from" {
  name  = "allow-notify-from"
  value = ["192.168.0.0/16", "10.0.0.0/8"]
}

resource "powerdns_recursor_forward_zone" "example" {
  zone    = "example.com."
  servers = ["pdns:5300"]
}

data "powerdns_record_soa" "soa" {
  zone = powerdns_zone.test.name
  name = powerdns_zone.test.name

  depends_on = [powerdns_record_soa.soa]
}

resource "powerdns_record_soa" "soa" {
  zone    = powerdns_zone.test.name
  name    = powerdns_zone.test.name
  ttl     = 3600
  mname   = "dns1.${powerdns_zone.test.name}"
  rname   = "hostmaster.${powerdns_zone.test.name}"
  refresh = 10800
  retry   = 3600
  expire  = 3600000
  minimum = 3600
}

resource "powerdns_zone" "test_disabled_soa" {
  name = "test-disabled-soa.example.com."
  kind = "Native"
}

resource "powerdns_record_soa" "disabled_zone" {
  zone     = powerdns_zone.test_disabled_soa.name
  name     = powerdns_zone.test_disabled_soa.name
  ttl      = 1800
  mname    = "dns1.${powerdns_zone.test_disabled_soa.name}"
  rname    = "hostmaster.${powerdns_zone.test_disabled_soa.name}"
  refresh  = 7200
  retry    = 1800
  expire   = 1209600
  minimum  = 600
  disabled = true
}

data "powerdns_record_soa" "disabled_zone" {
  zone = powerdns_zone.test_disabled_soa.name
  name = powerdns_zone.test_disabled_soa.name

  depends_on = [powerdns_record_soa.disabled_zone]
}

resource "powerdns_zone" "test2" {
  name = "test2.example.com."
  kind = "Native"
}

resource "powerdns_zone" "test_variant" {
  name = "test.example.com..internal"
  kind = "Native"
}

resource "powerdns_zone" "catalog" {
  name = "catalog-a.example."
  kind = "Producer"
}

resource "powerdns_zone" "catalog_member" {
  name    = "catalog-member.example.com."
  kind    = "Master"
  catalog = "catalog-a.example."

  depends_on = [powerdns_zone.catalog]
}

resource "powerdns_view_zone_association" "test2" {
  view = "internal"
  zone = powerdns_zone.test2.name
}

resource "powerdns_view_zone_association" "test_variant" {
  view = "internal"
  zone = powerdns_zone.test_variant.name
}

resource "powerdns_network" "internal_clients" {
  network = "192.0.2.0/24"
  view    = powerdns_view_zone_association.test2.view
}

variable "cryptokey_keytype" {
  type    = string
  default = "csk"
}

variable "cryptokey_active" {
  type    = bool
  default = true
}

variable "cryptokey_published" {
  type    = bool
  default = true
}

variable "cryptokey_split_signing" {
  type    = bool
  default = false
}

resource "powerdns_zone" "cryptokey" {
  name = "cryptokey.example.com."
  kind = "Native"
}

resource "powerdns_zone_cryptokey" "test" {
  zone      = powerdns_zone.cryptokey.name
  keytype   = var.cryptokey_keytype
  algorithm = "ecdsa256"
  active    = var.cryptokey_active
  published = var.cryptokey_published
}

resource "powerdns_zone_cryptokey" "split_zsk" {
  count = var.cryptokey_split_signing ? 1 : 0

  zone      = powerdns_zone.cryptokey.name
  keytype   = "zsk"
  algorithm = "ecdsa256"
}

data "powerdns_zone_cryptokeys" "test" {
  zone = powerdns_zone.cryptokey.name

  depends_on = [powerdns_zone_cryptokey.test, powerdns_zone_cryptokey.split_zsk]
}

resource "powerdns_zone" "cryptokey_rsa" {
  name = "cryptokey-rsa.example.com."
  kind = "Native"
}

resource "powerdns_zone_cryptokey" "rsa" {
  zone      = powerdns_zone.cryptokey_rsa.name
  keytype   = "ksk"
  algorithm = "rsasha256"
  bits      = 2048
}

variable "zone_actions_record_value" {
  type    = string
  default = "192.0.2.1"
}

resource "powerdns_zone" "zone_actions" {
  name = "zone-actions.example.com."
  kind = "Master"
}

resource "powerdns_record" "zone_actions_www" {
  zone    = powerdns_zone.zone_actions.name
  name    = "www.${powerdns_zone.zone_actions.name}"
  type    = "A"
  ttl     = 300
  records = [var.zone_actions_record_value]
}

# Prepare DNSSEC using the supported key resource, without a zone dnssec argument.
resource "powerdns_zone_cryptokey" "zone_actions" {
  zone      = powerdns_zone.zone_actions.name
  keytype   = "csk"
  algorithm = "ecdsa256"
}

resource "powerdns_zone_notify" "test" {
  zone = powerdns_zone.zone_actions.name

  triggers = {
    records = join(",", powerdns_record.zone_actions_www.records)
  }
}

resource "powerdns_zone_rectify" "test" {
  zone = powerdns_zone.zone_actions.name

  triggers = {
    records = join(",", powerdns_record.zone_actions_www.records)
  }

  depends_on = [powerdns_zone_cryptokey.zone_actions]
}

resource "powerdns_zone_notify" "once" {
  zone = powerdns_zone.zone_actions.name

  depends_on = [powerdns_record.zone_actions_www]
}

resource "powerdns_zone_rectify" "once" {
  zone = powerdns_zone.zone_actions.name

  depends_on = [powerdns_zone_cryptokey.zone_actions, powerdns_record.zone_actions_www]
}

variable "tsig_explicit_rotated" {
  type    = bool
  default = false
}

variable "tsig_migrate_to_write_only" {
  type    = bool
  default = false
}

variable "tsig_algorithm" {
  type    = string
  default = "hmac-sha256"
}

variable "tsig_wo_name" {
  type    = string
  default = "e2e-tsig-write-only"
}

variable "tsig_wo_secret_revision" {
  type    = number
  default = 1
}

variable "tsig_wo_version" {
  type    = number
  default = 1
}

locals {
  # Fixed, non-production secrets make rotation and state checks repeatable.
  tsig_explicit_secret  = base64encode(var.tsig_explicit_rotated ? "e2e-tsig-explicit-rotated-secret" : "e2e-tsig-explicit-initial-secret")
  tsig_wo_secret        = base64encode(var.tsig_wo_secret_revision == 1 ? "e2e-tsig-write-only-initial-secret" : var.tsig_wo_secret_revision == 2 ? "e2e-tsig-write-only-replacement-secret" : "e2e-tsig-write-only-rotated-secret")
  tsig_migration_secret = base64encode("e2e-tsig-generated-migration-secret")
}

resource "powerdns_tsigkey" "generated" {
  name           = "e2e-tsig-generated."
  algorithm      = "hmac-sha256"
  key_wo         = var.tsig_migrate_to_write_only ? local.tsig_migration_secret : null
  key_wo_version = var.tsig_migrate_to_write_only ? 1 : null
}

resource "powerdns_tsigkey" "explicit" {
  name           = "e2e-tsig-explicit"
  algorithm      = var.tsig_algorithm
  key            = var.tsig_migrate_to_write_only ? null : local.tsig_explicit_secret
  key_wo         = var.tsig_migrate_to_write_only ? local.tsig_explicit_secret : null
  key_wo_version = var.tsig_migrate_to_write_only ? 1 : null
}

resource "powerdns_tsigkey" "write_only" {
  name           = var.tsig_wo_name
  algorithm      = "hmac-sha256"
  key_wo         = local.tsig_wo_secret
  key_wo_version = var.tsig_wo_version
}

variable "tsig_scoped_algorithm" {
  type    = string
  default = "hmac-sha256"
}

resource "powerdns_tsigkey" "algorithm_scope" {
  name      = "e2e-tsig-algorithm-scope"
  algorithm = var.tsig_scoped_algorithm
}

# Reading a TSIG data source intentionally persists its secret. Remove this
# consumer when the generated resource switches to write-only mode.
data "powerdns_tsigkey" "generated" {
  count = var.tsig_migrate_to_write_only ? 0 : 1

  id = powerdns_tsigkey.generated.id
}

resource "powerdns_zone" "tsig" {
  name = "tsig.example.com."
  kind = "Master"
}

resource "powerdns_zone" "tsig_write_only" {
  name                = "tsig-write-only.example.com."
  kind                = "Master"
  master_tsig_key_ids = [powerdns_tsigkey.write_only.id]
  slave_tsig_key_ids  = [powerdns_tsigkey.write_only.id]
}

resource "powerdns_zone_metadata" "tsig_transfer" {
  zone     = powerdns_zone.tsig.name
  kind     = "TSIG-ALLOW-AXFR"
  metadata = [powerdns_tsigkey.generated.name]
}

variable "tsig_zone_reference_mode" {
  type    = string
  default = "bare"
}

resource "powerdns_tsigkey" "zone_reference" {
  name      = "e2e-zone-transfer"
  algorithm = "hmac-sha256"
}

resource "powerdns_tsigkey" "zone_reference_next" {
  name      = "e2e-zone-transfer-next"
  algorithm = "hmac-sha256"
}

locals {
  tsig_zone_key_id   = var.tsig_zone_reference_mode == "next" ? powerdns_tsigkey.zone_reference_next.id : powerdns_tsigkey.zone_reference.id
  tsig_zone_key_name = var.tsig_zone_reference_mode == "next" ? powerdns_tsigkey.zone_reference_next.name : powerdns_tsigkey.zone_reference.name
  tsig_zone_key_ref  = var.tsig_zone_reference_mode == "id" ? local.tsig_zone_key_id : local.tsig_zone_key_name
}

resource "powerdns_zone" "tsig_references" {
  name = "tsig-references.example.com."
  kind = "Master"

  # A bare name and its ID must identify one set element.
  master_tsig_key_ids = var.tsig_zone_reference_mode == "clear" ? [] : [local.tsig_zone_key_ref, local.tsig_zone_key_id]
  slave_tsig_key_ids  = var.tsig_zone_reference_mode == "clear" ? [] : [local.tsig_zone_key_ref]
}

data "powerdns_zone" "tsig_references" {
  name = powerdns_zone.tsig_references.name

  depends_on = [powerdns_zone.tsig_references]
}
