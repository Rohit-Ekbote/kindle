output "external_ip" {
  value = google_compute_address.env.address
}

output "vm_name" {
  value = google_compute_instance.env.name
}

output "fqdn" {
  value = "${var.env_name}.${var.zone_domain}"
}
