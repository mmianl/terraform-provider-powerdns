output "powerdns_zone_test" {
  value = powerdns_zone.test
}

output "data_powerdns_zone_test" {
  value = data.powerdns_zone.test
}

output "powerdns_zone_metadata_test_also_notify" {
  value = powerdns_zone_metadata.test_also_notify
}

output "powerdns_zone_metadata_test_allow_axfr_from" {
  value = powerdns_zone_metadata.test_allow_axfr_from
}

output "data_powerdns_zone_metadata_test_also_notify" {
  value = data.powerdns_zone_metadata.test_also_notify
}

output "data_powerdns_zone_metadata_list_test" {
  value = data.powerdns_zone_metadata_list.test
}

output "powerdns_reverse_zone_172_16_0_0_24" {
  value = powerdns_reverse_zone.zone_172_16_0_0_24
}

output "data_powerdns_reverse_zone_172_16_0_0_24" {
  value = data.powerdns_reverse_zone.zone_172_16_0_0_24
}

output "powerdns_record_host01" {
  value = powerdns_record.host01
}

output "powerdns_record_host02_disabled" {
  value = powerdns_record.host02_disabled
}

output "data_powerdns_record_host02_disabled" {
  value = data.powerdns_record.host02_disabled
}

output "data_powerdns_record_host02_disabled_disabled" {
  value = data.powerdns_record.host02_disabled.disabled
}

output "powerdns_ptr_record_host01_ipv4" {
  value = powerdns_ptr_record.host01_ipv4
}

output "powerdns_recursor_config_allow_from" {
  value = powerdns_recursor_config.allow_from
}

output "powerdns_recursor_config_allow_notify_from" {
  value = powerdns_recursor_config.allow_notify_from
}

output "powerdns_recursor_forward_zone_example" {
  value = powerdns_recursor_forward_zone.example
}

output "powerdns_record_soa" {
  value = powerdns_record_soa.soa
}

output "data_powerdns_record_soa" {
  value = data.powerdns_record_soa.soa
}

output "powerdns_zone_test_disabled_soa" {
  value = powerdns_zone.test_disabled_soa
}

output "powerdns_record_soa_disabled_zone" {
  value = powerdns_record_soa.disabled_zone
}

output "data_powerdns_record_soa_disabled_zone" {
  value = data.powerdns_record_soa.disabled_zone
}

output "data_powerdns_record_soa_disabled_zone_disabled" {
  value = data.powerdns_record_soa.disabled_zone.disabled
}

output "powerdns_zone_test_variant" {
  value = powerdns_zone.test_variant
}

output "powerdns_zone_catalog" {
  value = powerdns_zone.catalog
}

output "powerdns_zone_catalog_member" {
  value = powerdns_zone.catalog_member
}

output "powerdns_view_zone_association_test2" {
  value = powerdns_view_zone_association.test2
}

output "powerdns_view_zone_association_test_variant" {
  value = powerdns_view_zone_association.test_variant
}

output "powerdns_network_internal_clients" {
  value = powerdns_network.internal_clients
}

# Export public key attributes without the sensitive private-key content.
output "powerdns_zone_cryptokey_test" {
  value = {
    id        = powerdns_zone_cryptokey.test.id
    zone      = powerdns_zone_cryptokey.test.zone
    key_id    = powerdns_zone_cryptokey.test.key_id
    keytype   = powerdns_zone_cryptokey.test.keytype
    algorithm = powerdns_zone_cryptokey.test.algorithm
    bits      = powerdns_zone_cryptokey.test.bits
    active    = powerdns_zone_cryptokey.test.active
    published = powerdns_zone_cryptokey.test.published
    dnskey    = powerdns_zone_cryptokey.test.dnskey
    ds        = powerdns_zone_cryptokey.test.ds
    cds       = powerdns_zone_cryptokey.test.cds
  }
}

output "powerdns_zone_cryptokey_rsa" {
  value = {
    id        = powerdns_zone_cryptokey.rsa.id
    zone      = powerdns_zone_cryptokey.rsa.zone
    key_id    = powerdns_zone_cryptokey.rsa.key_id
    keytype   = powerdns_zone_cryptokey.rsa.keytype
    algorithm = powerdns_zone_cryptokey.rsa.algorithm
    bits      = powerdns_zone_cryptokey.rsa.bits
    active    = powerdns_zone_cryptokey.rsa.active
    published = powerdns_zone_cryptokey.rsa.published
    dnskey    = powerdns_zone_cryptokey.rsa.dnskey
    ds        = powerdns_zone_cryptokey.rsa.ds
    cds       = powerdns_zone_cryptokey.rsa.cds
  }
}

output "data_powerdns_zone_cryptokeys_test" {
  value = data.powerdns_zone_cryptokeys.test
}

output "powerdns_zone_actions" {
  value = {
    notify       = powerdns_zone_notify.test
    rectify      = powerdns_zone_rectify.test
    notify_once  = powerdns_zone_notify.once
    rectify_once = powerdns_zone_rectify.once
  }
}

# Public identities only; secret values are checked through state and the API.
output "powerdns_tsigkeys" {
  value = {
    generated = {
      id        = powerdns_tsigkey.generated.id
      name      = powerdns_tsigkey.generated.name
      algorithm = powerdns_tsigkey.generated.algorithm
    }
    explicit = {
      id        = powerdns_tsigkey.explicit.id
      name      = powerdns_tsigkey.explicit.name
      algorithm = powerdns_tsigkey.explicit.algorithm
    }
    write_only = {
      id        = powerdns_tsigkey.write_only.id
      name      = powerdns_tsigkey.write_only.name
      algorithm = powerdns_tsigkey.write_only.algorithm
    }
  }
}

output "powerdns_zone_metadata_tsig_transfer" {
  value = powerdns_zone_metadata.tsig_transfer
}

output "powerdns_zone_tsig_write_only" {
  value = powerdns_zone.tsig_write_only
}

output "powerdns_zone_tsig_references" {
  value = {
    zone = powerdns_zone.tsig_references
    data = data.powerdns_zone.tsig_references
  }
}
