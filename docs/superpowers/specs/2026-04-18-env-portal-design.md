# Env Management Portal — Design Spec

**Date:** 2026-04-18  
**Status:** Approved

---

## Context

An internal web portal for engineering teams to provision and manage application environments. Each environment ("env") is a single GCP VM running a single-node k3s cluster, with the application deployed as a versioned Helm chart. The portal is a convenience layer — engineers may continue to use `kubectl`, `helm`, or `gcloud` directly.

Key decisions made during brainstorming:

- **Backend:** Go (best-in-class `client-go`, Helm Go SDK, GCP client libraries)
- **Frontend:** React SPA (Vite), embedded into Go binary via `go:embed`
- **Auth:** Application-layer Google OAuth2, corporate domain enforced via `hd` claim
- **SSH to VMs:** IAP tunneling (no public SSH port needed)
- **Storage:** Fully local (SQLite for event log, local filesystem for Terraform state + configs)
- **Templates:** Stored in portal repo under `presets/` (not the Helm chart repo — chart is customer-facing)
- **Portal deployment:** GCE VM; code is deployment-agnostic
- **Status:** On-demand fetch via button (not continuous polling per row)
- **Terraform:** Shelled out as subprocess with local state

---

## Section 1: Overall Architecture

A single Go binary serves both the REST API and the embedded React SPA.

```
portal (Go binary)
├── HTTP server
│   ├── /auth/*           OAuth2 login/callback
│   ├── /api/*            REST API handlers
│   └── /*                embedded React SPA (go:embed)
├── Internal packages
│   ├── auth/             Google OAuth2 + session middleware
│   ├── api/              HTTP handlers (one file per resource group)
│   ├── gcp/              Compute API client (VM list, get, labels)
│   ├── k8s/              client-go (workloads, images)
│   ├── helm/             shell-out helm runner
│   ├── terraform/        shell-out runner + in-memory job map
│   ├── iapssh/           SSH-over-IAP client (kubeconfig fetch)
│   ├── presets/          template loader (reads from ./presets/ dir)
│   ├── chartversions/    git ls-remote against public chart repo
│   └── db/               SQLite event log
└── Local data layout
    data/envs/<env-name>/
    ├── terraform/        working dir (tfvars, state, module symlink)
    ├── values.yaml       Helm override values
    └── ops/<ts>-<action>.log
```

### Repo Layout

```
cmd/portal/         Go entrypoint
internal/           all Go packages
web/                React SPA source (Vite, built → web/dist → embedded)
presets/            env preset templates
terraform/module/   reusable Terraform root module
docs/
```

---

## Section 2: Data Flow for Key Operations

### Env List (`GET /api/envs`)
GCP Compute API → filter VMs by label `managed-by=env-portal` → return metadata (name, owner, machine type, zone, created-at). Status column shows last known outcome from in-memory job map. A "Refresh Status" button per row triggers an on-demand status call.

### On-demand Status (`GET /api/envs/{name}/status`)
IAP SSH → k8s API reachability check + `helm status` → return `ready | degraded | provisioning | deleting | failed`.

### Create Env (`POST /api/envs`)
1. Validate name uniqueness against live GCP VM list
2. Write `data/envs/<name>/terraform/terraform.tfvars` + `values.yaml` from preset + user overrides
3. Copy Terraform module into `data/envs/<name>/terraform/`
4. Spawn goroutine: `terraform init && terraform apply` → stream output to `ops/<ts>-create.log`
5. Update in-memory job map → `provisioning`
6. Return 202 immediately; UI polls `GET /api/envs/{name}` every 5s

### Kubeconfig (`GET /api/envs/{name}/kubeconfig`)
IAP SSH to VM → read `/etc/rancher/k3s/k3s.yaml` → replace `server: https://127.0.0.1:6443` with `https://<name>.<zone-domain>:6443` → return as `<name>-kubeconfig.yaml`.

### Edit Values (`POST /api/envs/{name}/edit-values`)
Write updated `values.yaml` to disk → shell-out `helm upgrade` against env's cluster (using cached kubeconfig) → write event to SQLite.

### Update Image Tag (`POST /api/envs/{name}/update-image`)
Each preset declares image tag key paths in `values.yaml` using the convention `<workload-name>.image.tag` (e.g., `api.image.tag`). The portal updates the matching key in the env's `values.yaml` on disk → `helm upgrade` → write event to SQLite. Presets must declare `image_tag_keys` so the portal knows which values keys map to which workloads.

### Upgrade Chart (`POST /api/envs/{name}/upgrade-chart`)
Update `chart_ref` in `terraform.tfvars` → spawn goroutine: `terraform apply` → update job map → write event.

### Resize VM (`POST /api/envs/{name}/resize`)
Update `machine_type` / `disk_size_gb` in `terraform.tfvars` → spawn goroutine: `terraform apply` → update job map → write event.

### Delete Env (`DELETE /api/envs/{name}`)
Spawn goroutine: `terraform destroy` → on success, move `data/envs/<name>/` to `data/archive/<name>/` → update job map → write final event.

### List Chart Versions (`GET /api/templates/{name}/chart-versions`)
`git ls-remote <chart-repo-url>` → return tags + branches.

---

## Section 3: Auth, Session Handling & Terraform Job Runner

### Google OAuth2 Auth

- Unauthenticated request → redirect to `/auth/login` → Google OAuth2 with `hd=<corporate-domain>` hint
- `/auth/callback` → verify `hd` claim matches corporate domain; reject anything outside it
- On success → signed session cookie via `gorilla/sessions` containing `{ email, name, expires_at }`
- All `/api/*` routes protected by session middleware
- `/auth/*` and `/healthz` are public

### Terraform Job Runner

```go
type JobStatus struct {
    State     string    // provisioning | deleting | idle
    StartedAt time.Time
    Actor     string    // email of who triggered it
    LogPath   string    // path to ops log file
    Err       error     // set on failure
}

type Runner struct {
    mu   sync.RWMutex
    jobs map[string]*JobStatus  // keyed by env name
}
```

- One goroutine per in-flight operation; mutex protects the map
- Goroutine streams `terraform` stdout+stderr to `ops/<ts>-<action>.log`
- On completion: updates job map, writes success/failure event to SQLite
- On portal restart: in-memory map is empty; state is clean (local Terraform state has no distributed locks)
- Only one operation per env at a time — second request returns `409 Conflict`

### Cached Kubeconfig

- In-memory `map[string][]byte` of kubeconfig per env, fetched on first use via IAP SSH
- Used by both `helm upgrade` calls and `client-go` for workload listing
- Invalidated when a create/delete operation completes for that env, or on portal restart

---

## Section 4: Frontend Structure & Preset Template Schema

### React SPA Structure

```
web/src/
├── pages/
│   ├── EnvList.tsx          list view, search/sort, Create button
│   ├── EnvDetail.tsx        detail view, actions, event log
│   └── CreateEnv.tsx        form: name, preset, overrides
├── components/
│   ├── WorkloadsTable.tsx   deployments/statefulsets/daemonsets
│   ├── ImageTagModal.tsx    inline edit for image tags
│   ├── ValuesEditor.tsx     YAML editor (monaco-editor)
│   ├── StatusBadge.tsx      ready/degraded/provisioning/etc
│   └── EventLog.tsx         chronological action feed
├── api/                     typed fetch wrappers for each endpoint
└── hooks/                   useEnvStatus, useTemplates, useChartVersions
```

- **Polling:** After triggering create/delete/upgrade/resize, the detail page polls `GET /api/envs/{name}` every 5s until status leaves `provisioning`/`deleting`
- **YAML editor:** `monaco-editor` for values editing (syntax highlighting, validation)
- **Build:** Vite → `web/dist` → embedded into Go binary via `go:embed web/dist`

### Preset Template Schema

`presets/<name>/template.yaml`:

```yaml
name: small-dev
description: "Lightweight dev env"
defaults:
  machine_type: e2-standard-2
  disk_size_gb: 50
  zone: us-central1-a
  chart_ref: main

image_tag_keys:
  api: api.image.tag
  worker: worker.image.tag

user_editable:
  - key: machine_type
    label: "VM Machine Type"
    type: enum
    options: [e2-standard-2, e2-standard-4, e2-standard-8]
  - key: disk_size_gb
    label: "Disk Size (GB)"
    type: integer
    min: 20
    max: 500
  - key: app.replicas
    label: "App Replicas"
    type: integer
    min: 1
    max: 5
```

`presets/<name>/values.yaml` holds the baseline Helm values for the preset. User overrides are deep-merged on top at create time.

---

## Section 5: Data Model

### SQLite Schema

```sql
CREATE TABLE env_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    env_name    TEXT NOT NULL,
    actor_email TEXT NOT NULL,
    action_type TEXT NOT NULL,  -- create | upgrade_chart | edit_values | update_image | resize_vm | delete
    description TEXT NOT NULL,
    outcome     TEXT NOT NULL,  -- success | failure | in_progress
    log_path    TEXT,           -- local path to terraform/helm output log
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_env_events_env_name ON env_events(env_name, created_at DESC);
```

### Local Filesystem Layout

```
data/
  envs/
    <env-name>/
      terraform/
        terraform.tfvars
        terraform.tfstate
        (module files copied from terraform/module/)
      values.yaml
      ops/
        <ts>-create.log
        <ts>-upgrade_chart.log
        ...
  archive/
    <env-name>/       (moved here on deletion)
```

### Config (environment variables / config file)

```
GOOGLE_CLIENT_ID
GOOGLE_CLIENT_SECRET
CORPORATE_DOMAIN          e.g. company.com
ZONE_DOMAIN               e.g. local-dev.example.com
GCP_PROJECT_ID
CHART_REPO_URL            public git URL
SESSION_SECRET            random 32-byte secret (generate with: openssl rand -hex 32)
DATA_DIR                  default: ./data
PRESETS_DIR               default: ./presets
```

---

## Section 6: API Surface

All endpoints require authenticated session except `/auth/*` and `/healthz`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/envs` | List envs (live GCP query) |
| `GET` | `/api/envs/{name}` | Env detail (live GCP + job map) |
| `GET` | `/api/envs/{name}/status` | On-demand k8s + Helm status |
| `POST` | `/api/envs` | Create env |
| `POST` | `/api/envs/{name}/upgrade-chart` | Upgrade chart version |
| `POST` | `/api/envs/{name}/edit-values` | Edit Helm values |
| `POST` | `/api/envs/{name}/update-image` | Update workload image tag |
| `POST` | `/api/envs/{name}/resize` | Resize VM |
| `DELETE` | `/api/envs/{name}` | Delete env |
| `GET` | `/api/envs/{name}/kubeconfig` | Download kubeconfig |
| `GET` | `/api/envs/{name}/events` | Paginated event log |
| `GET` | `/api/templates` | List available presets |
| `GET` | `/api/templates/{name}/chart-versions` | List chart tags + branches |
| `GET` | `/healthz` | Health check |

---

## Out of Scope (v1)

- GCS/Cloud SQL — all storage is local; crash recovery deferred
- Multi-target env support (GKE, VM pools)
- Cost tracking, scheduled teardown, TTL on envs
- Live log streaming for operations
- RBAC, team-based ownership
- Concurrent-operation safety beyond 409 per-env lock
- Rollback-to-previous-values as a first-class button
