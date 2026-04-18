variable "env_name" {
  type        = string
  description = "Unique name for this environment"
}

variable "owner_email" {
  type        = string
  description = "Email of the engineer who created this env"
}

variable "template_name" {
  type        = string
  description = "Name of the preset template used"
}

variable "machine_type" {
  type    = string
  default = "e2-standard-2"
}

variable "disk_size_gb" {
  type    = number
  default = 50
}

variable "zone" {
  type    = string
  default = "us-central1-a"
}

variable "project" {
  type        = string
  description = "GCP project ID"
}

variable "chart_ref" {
  type        = string
  default     = "main"
  description = "Git ref (tag or branch) for the Helm chart"
}

variable "chart_repo_url" {
  type        = string
  description = "Public URL of the Helm chart git repository"
}

variable "zone_domain" {
  type        = string
  description = "DNS zone domain, e.g. local-dev.example.com"
}

variable "dns_zone_name" {
  type        = string
  description = "Cloud DNS managed zone name, e.g. local-dev"
}

variable "dns_project" {
  type        = string
  description = "GCP project containing the Cloud DNS zone"
}

variable "iap_cidr" {
  type        = string
  default     = "35.235.240.0/20"
  description = "Google IAP CIDR for SSH tunnel access"
}

variable "k3s_api_cidr" {
  type        = string
  description = "Corporate CIDR allowed to reach the k3s API (port 6443)"
}
