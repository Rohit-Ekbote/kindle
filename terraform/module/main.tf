terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project
  zone    = var.zone
}

locals {
  env_labels = {
    managed-by = "env-portal"
    env-name   = var.env_name
    owner      = replace(var.owner_email, "@", "_at_")
    template   = var.template_name
  }
  fqdn = "${var.env_name}.${var.zone_domain}"
}

resource "google_compute_address" "env" {
  name   = "${var.env_name}-ip"
  region = join("-", slice(split("-", var.zone), 0, 2))
}

resource "google_compute_instance" "env" {
  name         = var.env_name
  machine_type = var.machine_type
  zone         = var.zone
  labels       = local.env_labels

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = var.disk_size_gb
    }
  }

  network_interface {
    network = "default"
    access_config {
      nat_ip = google_compute_address.env.address
    }
  }

  metadata = {
    enable-oslogin = "TRUE"
    startup-script = templatefile("${path.module}/startup.sh", {
      env_name       = var.env_name
      chart_ref      = var.chart_ref
      chart_repo_url = var.chart_repo_url
      fqdn           = local.fqdn
    })
  }

  service_account {
    scopes = ["cloud-platform"]
  }
}

resource "google_compute_firewall" "k3s_api" {
  name    = "${var.env_name}-k3s-api"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["6443"]
  }

  source_ranges = [var.k3s_api_cidr]
  target_tags   = ["${var.env_name}-env"]
}

resource "google_compute_firewall" "iap_ssh" {
  name    = "${var.env_name}-iap-ssh"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = [var.iap_cidr]
  target_tags   = ["${var.env_name}-env"]
}

resource "google_dns_record_set" "env" {
  name         = "${local.fqdn}."
  type         = "A"
  ttl          = 300
  managed_zone = var.dns_zone_name
  project      = var.dns_project
  rrdatas      = [google_compute_address.env.address]
}
