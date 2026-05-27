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
