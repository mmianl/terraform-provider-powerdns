resource "powerdns_zone" "test" {
  name         = "test.example.com."
  kind         = "Native"
  soa_edit_api = "DEFAULT"
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
  zone    = powerdns_zone.test.name
  name    = "host01.test.example.com."
  type    = "A"
  ttl     = 30
  records = [cidrhost("172.16.0.0/24", 10)]
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

resource "powerdns_zone" "test2" {
  name = "test2.example.com."
  kind = "Native"
}
