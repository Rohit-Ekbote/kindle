# Secrets Integration Design

## Goal

Automatically fetch secrets from a configured backend (HashiCorp Vault or GCP Secret Manager) at Helm upgrade time and inject them as values, eliminating manual secret handling during environment provisioning.

## Architecture

A `SecretFetcher` interface abstracts the secret backend. At startup the portal reads an embedded config file to determine which implementation to instantiate. At `helm upgrade` time the fetcher retrieves all configured secrets and passes them to the Helm runner via a short-lived temp file. Three implementations cover all cases: `VaultFetcher`, `GCPFetcher`, and `NoOpFetcher`.

## Tech Stack

- Go `go:embed` for bundling `secrets.yaml`
- HashiCorp Vault KV v2 API (HTTP)
- GCP Secret Manager API (`cloud.google.com/go/secretmanager`)
- Application Default Credentials (ADC) for GCP auth — already mounted in container

---

## Config File

`internal/secrets/secrets.yaml` — embedded via `go:embed`, lives in the repo, no code changes needed to add/remove secrets:

```yaml
backend: vault   # vault | gcp | none
secrets:
  - openai-api-key
  - stripe-secret-key
```

`backend: none` (or omitted) activates `NoOpFetcher`. Valid values: `vault`, `gcp`, `none`.

---

## Interface

```go
// internal/secrets/fetcher.go
type SecretFetcher interface {
    FetchSecrets(ctx context.Context, envName string) (map[string]string, error)
}
```

`envName` is passed to every implementation. Current implementations use a shared path strategy (ignore `envName`), but the parameter ensures switching to per-env paths requires only an implementation change, not a caller change.

---

## Implementations

### VaultFetcher

**Required env vars:**
- `VAULT_ADDR` — e.g. `https://vault.example.com`
- `VAULT_TOKEN` — static token with read access
- `VAULT_SECRET_PATH_PREFIX` — optional, defaults to `secret/data`

**Behavior:** For each secret name in config, fetches `GET <VAULT_ADDR>/<VAULT_SECRET_PATH_PREFIX>/<name>`. Expects KV v2 response shape: `data.data.value`. Fails fast if any secret returns 404 or auth error, reporting the secret name.

### GCPFetcher

**Required env vars:**
- `GCP_PROJECT_ID` — already required by the portal

**Behavior:** Uses ADC (mounted at `/root/.config/gcloud` in the container). For each secret name, accesses `projects/<GCP_PROJECT_ID>/secrets/<name>/versions/latest`. Fails fast on missing secret or permission error.

### NoOpFetcher

Returns `map[string]string{}`, no error. Used when `backend: none` or config file is absent.

---

## Helm Integration

`UpgradeParams` gains a `SecretValues map[string]string` field. The Helm runner writes this map to a temp YAML file and appends it as an additional `--values` arg:

```go
type UpgradeParams struct {
    // ... existing fields
    SecretValues map[string]string
}
```

Call sequence in the env provisioner:

1. `fetcher.FetchSecrets(ctx, envName)` → `map[string]string`
2. `f, _ := os.CreateTemp("", "secrets-*.yaml")`
3. `defer os.Remove(f.Name())` — deleted even on panic/error
4. Marshal map to YAML, write to `f`
5. Append `--values <f.Name()>` to helm upgrade args

**Precedence:** Secret values are passed before user-supplied override values so user overrides win (last `--values` in Helm takes highest precedence).

---

## Error Handling

| Scenario | Behavior |
|---|---|
| Secret not found | Fail upgrade, return error naming the missing secret |
| Auth failure (Vault/GCP) | Fail upgrade, surface auth error |
| Partial fetch (N of M succeed) | Fail fast on first error, do not proceed with partial values |
| `NoOpFetcher` | No fetch, no temp file, upgrade proceeds normally |
| Temp file write failure | Fail upgrade, OS error surfaced |

---

## Startup Wiring

```go
fetcher, err := secrets.NewFetcherFromConfig()
// NewFetcherFromConfig reads embedded secrets.yaml,
// selects implementation based on backend field,
// validates required env vars are present,
// returns error at startup (not at request time) if misconfigured
```

The `fetcher` is injected into the env provisioner. Misconfiguration (e.g. `backend: vault` but `VAULT_ADDR` unset) fails at startup, not silently at provision time.

---

## File Structure

| Path | Purpose |
|---|---|
| `internal/secrets/secrets.yaml` | Embedded config: backend type + secret names |
| `internal/secrets/fetcher.go` | `SecretFetcher` interface + `NewFetcherFromConfig` |
| `internal/secrets/vault.go` | `VaultFetcher` implementation |
| `internal/secrets/gcp.go` | `GCPFetcher` implementation |
| `internal/secrets/noop.go` | `NoOpFetcher` implementation |
| `internal/helm/runner.go` | Modified to accept `SecretValues`, write temp file |
