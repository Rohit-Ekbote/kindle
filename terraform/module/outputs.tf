output "external_ip" {
  value = google_compute_address.env.address
}

output "vm_name" {
  value = google_compute_instance.env.name
}

output "fqdn" {
  value = local.fqdn
}
