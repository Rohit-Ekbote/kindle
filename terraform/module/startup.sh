#!/bin/bash
set -euo pipefail

ENV_NAME="${env_name}"
CHART_REF="${chart_ref}"
CHART_REPO_URL="${chart_repo_url}"
FQDN="${fqdn}"

# Install k3s with TLS SAN for the DNS hostname
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--tls-san $FQDN" sh -

# Wait for k3s to be ready
until kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml get nodes 2>/dev/null | grep -q Ready; do
  sleep 5
done

# Install Helm
curl -sfL https://raw.githubusercontent.com/helm/helm/v3.17.3/scripts/get-helm-3 | bash

# Clone chart repo at specified ref
git clone --depth 1 --branch "$CHART_REF" "$CHART_REPO_URL" /opt/chart-repo

# Deploy Helm chart
helm upgrade --install "$ENV_NAME" /opt/chart-repo \
  --namespace "$ENV_NAME" \
  --create-namespace \
  --values /opt/portal-values.yaml \
  --kubeconfig /etc/rancher/k3s/k3s.yaml \
  --wait --timeout 10m
