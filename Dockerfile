# ── Stage 1: Build the React frontend ────────────────────────────────────────
FROM node:24-alpine AS frontend

WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ── Stage 2: Build the Go binary ─────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy Go module files and download dependencies before copying source
# so this layer is cached as long as go.mod/go.sum don't change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy the compiled React SPA so go:embed finds it at cmd/portal/web/dist
COPY --from=frontend /app/web/dist ./cmd/portal/web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /portal ./cmd/portal

# ── Stage 3: Runtime image ───────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

# Install runtime dependencies:
#   curl/gnupg     — needed to add apt repos below
#   git            — used by chartversions.Lister to list chart refs
#   openssh-client — used by iapssh to establish SSH connections
#   unzip          — needed by Terraform installer
#   google-cloud-sdk — provides gcloud (IAP tunnel, Compute API) and gke-gcloud-auth-plugin
#   terraform      — shelled out by terraform.Runner
#   helm           — shelled out by helm.Runner
RUN apt-get update && apt-get install -y --no-install-recommends \
        curl \
        gnupg \
        git \
        openssh-client \
        unzip \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Google Cloud SDK
RUN curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg \
        | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" \
        > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update && apt-get install -y --no-install-recommends \
        google-cloud-sdk \
        google-cloud-sdk-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/*

# Terraform
ARG TERRAFORM_VERSION=1.10.5
RUN curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" \
        -o /tmp/terraform.zip \
    && unzip /tmp/terraform.zip -d /usr/local/bin \
    && rm /tmp/terraform.zip \
    && chmod +x /usr/local/bin/terraform

# Helm
ARG HELM_VERSION=3.17.3
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
        | tar -xz --strip-components=1 -C /usr/local/bin linux-amd64/helm \
    && chmod +x /usr/local/bin/helm

# Copy the portal binary
COPY --from=builder /portal /usr/local/bin/portal

# Data directory for SQLite DB and Terraform state
VOLUME ["/data"]

# Presets directory (mount a ConfigMap or host path with preset YAML files)
VOLUME ["/presets"]

ENV DATA_DIR=/data \
    PRESETS_DIR=/presets \
    PORT=8080

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/portal"]
