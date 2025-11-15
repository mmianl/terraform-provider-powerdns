resource "powerdns_zone" "test" {
  name = "test.example.com."
  kind = "Native"
  nameservers = [
    "ns1.example.com.",
    "ns2.example.com.",
  ]
}

data "powerdns_zone" "test" {
  name = "test.example.com."

  depends_on = [powerdns_zone.test]
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
