# Secrets Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically fetch secrets from Vault or GCP Secret Manager at Helm upgrade time and inject them as values, eliminating manual secret handling during environment provisioning.

**Architecture:** A `SecretFetcher` interface in `internal/secrets` abstracts the backend. `NewFetcherFromConfig` reads an embedded `secrets.yaml` at startup and instantiates `VaultFetcher`, `GCPFetcher`, or `NoOpFetcher`. The fetcher is injected into `server.New`, which calls it inside the `HelmUpgrade` closure; the helm runner writes fetched values to a short-lived temp file passed as an additional `--values` arg.

**Tech Stack:** Go `go:embed`, `net/http` (Vault KV v2), `cloud.google.com/go/secretmanager/apiv1` (GCP), `gopkg.in/yaml.v3`, `testify`, `net/http/httptest`

---

## File Structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/secrets/secrets.yaml` | Create | Embedded config: backend type + secret name list |
| `internal/secrets/fetcher.go` | Create | `SecretFetcher` interface, `secretsConfig`, embed, `NewFetcherFromConfig` |
| `internal/secrets/noop.go` | Create | `NoOpFetcher` — returns empty map, no error |
| `internal/secrets/vault.go` | Create | `VaultFetcher` — Vault KV v2 via HTTP |
| `internal/secrets/gcp.go` | Create | `GCPFetcher` — Secret Manager via ADC |
| `internal/secrets/fetcher_test.go` | Create | Tests for `NewFetcherFromConfig` dispatch and `NoOpFetcher` |
| `internal/secrets/vault_test.go` | Create | `VaultFetcher` tests using `httptest.NewServer` |
| `internal/secrets/gcp_test.go` | Create | `GCPFetcher` constructor validation tests |
| `internal/helm/runner.go` | Modify | Add `SecretValues map[string]string` to `UpgradeParams`; write temp file in `Upgrade` |
| `internal/helm/runner_test.go` | Modify | Add test verifying `--values` appears twice when `SecretValues` provided |
| `internal/server/server.go` | Modify | Accept `secrets.SecretFetcher` param; call `FetchSecrets` in `HelmUpgrade` closure |
| `cmd/portal/main.go` | Modify | Create fetcher via `NewFetcherFromConfig`, pass to `server.New` |

---

### Task 1: SecretFetcher interface + NoOpFetcher

**Files:**
- Create: `internal/secrets/fetcher.go`
- Create: `internal/secrets/noop.go`
- Create: `internal/secrets/fetcher_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/secrets/fetcher_test.go
package secrets_test

import (
	"context"
	"testing"

	"github.com/emdash/kindle/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOpFetcher_ReturnsEmptyMap(t *testing.T) {
	f := secrets.NewNoOpFetcher()
	got, err := f.FetchSecrets(context.Background(), "my-env")
	require.NoError(t, err)
	assert.Empty(t, got)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/secrets/... -run TestNoOpFetcher_ReturnsEmptyMap -v`
Expected: FAIL — package does not exist yet

- [ ] **Step 3: Create the interface and NoOpFetcher**

```go
// internal/secrets/fetcher.go
package secrets

import "context"

type SecretFetcher interface {
	FetchSecrets(ctx context.Context, envName string) (map[string]string, error)
}
```

```go
// internal/secrets/noop.go
package secrets

import "context"

type NoOpFetcher struct{}

func NewNoOpFetcher() *NoOpFetcher { return &NoOpFetcher{} }

func (f *NoOpFetcher) FetchSecrets(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/secrets/... -run TestNoOpFetcher_ReturnsEmptyMap -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/fetcher.go internal/secrets/noop.go internal/secrets/fetcher_test.go
git commit -m "feat: add SecretFetcher interface and NoOpFetcher"
```

---

### Task 2: Embedded config + NewFetcherFromConfig

**Files:**
- Create: `internal/secrets/secrets.yaml`
- Modify: `internal/secrets/fetcher.go` (add embed + NewFetcherFromConfig)
- Modify: `internal/secrets/fetcher_test.go` (add dispatch tests)

- [ ] **Step 1: Write the failing tests**

Add to `internal/secrets/fetcher_test.go`:

```go
func TestNewFetcherFromConfig_NoneBackend(t *testing.T) {
	f, err := secrets.NewFetcherFromConfig()
	require.NoError(t, err)
	_, ok := f.(*secrets.NoOpFetcher)
	assert.True(t, ok, "expected NoOpFetcher for 'none' backend")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/secrets/... -run TestNewFetcherFromConfig_NoneBackend -v`
Expected: FAIL — `NewFetcherFromConfig` undefined

- [ ] **Step 3: Create secrets.yaml and implement NewFetcherFromConfig**

```yaml
# internal/secrets/secrets.yaml
backend: none
secrets: []
```

Replace `internal/secrets/fetcher.go` with:

```go
// internal/secrets/fetcher.go
package secrets

import (
	"context"
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed secrets.yaml
var configData []byte

type SecretFetcher interface {
	FetchSecrets(ctx context.Context, envName string) (map[string]string, error)
}

type secretsConfig struct {
	Backend string   `yaml:"backend"`
	Secrets []string `yaml:"secrets"`
}

func NewFetcherFromConfig() (SecretFetcher, error) {
	var cfg secretsConfig
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("parse secrets config: %w", err)
	}
	switch cfg.Backend {
	case "vault":
		return newVaultFetcher(cfg.Secrets)
	case "gcp":
		return newGCPFetcher(cfg.Secrets)
	case "none", "":
		return &NoOpFetcher{}, nil
	default:
		return nil, fmt.Errorf("unknown secret backend: %q", cfg.Backend)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/secrets/... -run TestNewFetcherFromConfig_NoneBackend -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/secrets.yaml internal/secrets/fetcher.go internal/secrets/fetcher_test.go
git commit -m "feat: add embedded secrets config and NewFetcherFromConfig"
```

---

### Task 3: VaultFetcher

**Files:**
- Create: `internal/secrets/vault.go`
- Create: `internal/secrets/vault_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/secrets/vault_test.go
package secrets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/emdash/kindle/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVaultFetcher_MissingAddr(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "tok")
	_, err := secrets.NewVaultFetcher([]string{"openai-key"})
	assert.ErrorContains(t, err, "VAULT_ADDR")
}

func TestVaultFetcher_MissingToken(t *testing.T) {
	t.Setenv("VAULT_ADDR", "http://vault:8200")
	t.Setenv("VAULT_TOKEN", "")
	_, err := secrets.NewVaultFetcher([]string{"openai-key"})
	assert.ErrorContains(t, err, "VAULT_TOKEN")
}

func TestVaultFetcher_FetchSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "mytoken", r.Header.Get("X-Vault-Token"))
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{"value": "sk-test123"},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "mytoken")
	t.Setenv("VAULT_SECRET_PATH_PREFIX", "secret/data")

	f, err := secrets.NewVaultFetcher([]string{"openai-key"})
	require.NoError(t, err)

	got, err := f.FetchSecrets(context.Background(), "my-env")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"openai-key": "sk-test123"}, got)
}

func TestVaultFetcher_SecretNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	t.Setenv("VAULT_ADDR", srv.URL)
	t.Setenv("VAULT_TOKEN", "tok")

	f, err := secrets.NewVaultFetcher([]string{"missing-secret"})
	require.NoError(t, err)

	_, err = f.FetchSecrets(context.Background(), "my-env")
	assert.ErrorContains(t, err, "missing-secret")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/secrets/... -run TestVaultFetcher -v`
Expected: FAIL — `NewVaultFetcher` undefined

- [ ] **Step 3: Implement VaultFetcher**

```go
// internal/secrets/vault.go
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type VaultFetcher struct {
	addr       string
	token      string
	pathPrefix string
	secrets    []string
}

func NewVaultFetcher(secrets []string) (*VaultFetcher, error) {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("VAULT_ADDR is required for vault backend")
	}
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("VAULT_TOKEN is required for vault backend")
	}
	prefix := os.Getenv("VAULT_SECRET_PATH_PREFIX")
	if prefix == "" {
		prefix = "secret/data"
	}
	return &VaultFetcher{addr: addr, token: token, pathPrefix: prefix, secrets: secrets}, nil
}

func (f *VaultFetcher) FetchSecrets(ctx context.Context, _ string) (map[string]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	result := make(map[string]string, len(f.secrets))
	for _, name := range f.secrets {
		url := fmt.Sprintf("%s/v1/%s/%s", f.addr, f.pathPrefix, name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request for secret %q: %w", name, err)
		}
		req.Header.Set("X-Vault-Token", f.token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch secret %q: %w", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch secret %q: vault returned HTTP %d", name, resp.StatusCode)
		}
		var body struct {
			Data struct {
				Data map[string]any `json:"data"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("decode secret %q: %w", name, err)
		}
		val, ok := body.Data.Data["value"]
		if !ok {
			return nil, fmt.Errorf("secret %q is missing key \"value\" in vault response", name)
		}
		strVal, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("secret %q value is not a string", name)
		}
		result[name] = strVal
	}
	return result, nil
}
```

Also update `fetcher.go` to call `NewVaultFetcher` (rename from `newVaultFetcher` since it's now exported for tests):

In `fetcher.go`, change:
```go
case "vault":
    return newVaultFetcher(cfg.Secrets)
```
to:
```go
case "vault":
    return NewVaultFetcher(cfg.Secrets)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/secrets/... -run TestVaultFetcher -v`
Expected: 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/vault.go internal/secrets/vault_test.go internal/secrets/fetcher.go
git commit -m "feat: add VaultFetcher with KV v2 HTTP client"
```

---

### Task 4: GCPFetcher + secretmanager dependency

**Files:**
- Create: `internal/secrets/gcp.go`
- Create: `internal/secrets/gcp_test.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

- [ ] **Step 1: Add the secretmanager dependency**

Run: `go get cloud.google.com/go/secretmanager/apiv1`
Expected: go.mod and go.sum updated, no error

- [ ] **Step 2: Write the failing tests**

```go
// internal/secrets/gcp_test.go
package secrets_test

import (
	"os"
	"testing"

	"github.com/emdash/kindle/internal/secrets"
	"github.com/stretchr/testify/assert"
)

func TestGCPFetcher_MissingProjectID(t *testing.T) {
	orig := os.Getenv("GCP_PROJECT_ID")
	os.Unsetenv("GCP_PROJECT_ID")
	t.Cleanup(func() { os.Setenv("GCP_PROJECT_ID", orig) })

	_, err := secrets.NewGCPFetcher([]string{"openai-key"})
	assert.ErrorContains(t, err, "GCP_PROJECT_ID")
}

func TestGCPFetcher_EmptySecrets_NoError(t *testing.T) {
	t.Setenv("GCP_PROJECT_ID", "my-project")
	// NewGCPFetcher with empty list should succeed without dialing GCP
	// (client is created lazily per FetchSecrets call when secrets is empty)
	f, err := secrets.NewGCPFetcher([]string{})
	assert.NoError(t, err)
	assert.NotNil(t, f)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/secrets/... -run TestGCPFetcher -v`
Expected: FAIL — `NewGCPFetcher` undefined

- [ ] **Step 4: Implement GCPFetcher**

```go
// internal/secrets/gcp.go
package secrets

import (
	"context"
	"fmt"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type GCPFetcher struct {
	project string
	secrets []string
}

func NewGCPFetcher(secrets []string) (*GCPFetcher, error) {
	project := os.Getenv("GCP_PROJECT_ID")
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID is required for gcp backend")
	}
	return &GCPFetcher{project: project, secrets: secrets}, nil
}

func (f *GCPFetcher) FetchSecrets(ctx context.Context, _ string) (map[string]string, error) {
	if len(f.secrets) == 0 {
		return map[string]string{}, nil
	}
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create secret manager client: %w", err)
	}
	defer client.Close()

	result := make(map[string]string, len(f.secrets))
	for _, name := range f.secrets {
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", f.project, name),
		}
		resp, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("fetch secret %q: %w", name, err)
		}
		result[name] = string(resp.Payload.Data)
	}
	return result, nil
}
```

Also update `fetcher.go` to call `NewGCPFetcher`:

In `fetcher.go`, change:
```go
case "gcp":
    return newGCPFetcher(cfg.Secrets)
```
to:
```go
case "gcp":
    return NewGCPFetcher(cfg.Secrets)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/secrets/... -run TestGCPFetcher -v`
Expected: 2 tests PASS

Run: `go test ./internal/secrets/... -v`
Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/secrets/gcp.go internal/secrets/gcp_test.go internal/secrets/fetcher.go go.mod go.sum
git commit -m "feat: add GCPFetcher using Secret Manager ADC"
```

---

### Task 5: Helm SecretValues temp file

**Files:**
- Modify: `internal/helm/runner.go`
- Modify: `internal/helm/runner_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/helm/runner_test.go`:

```go
func TestRunner_Upgrade_WithSecretValues(t *testing.T) {
	var capturedArgs []string
	dir := t.TempDir()
	script := filepath.Join(dir, "helm")
	// Capture all args to a file so we can inspect them
	captureFile := filepath.Join(dir, "args.txt")
	content := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\nexit 0\n", captureFile)
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(kubeconfigFile, []byte("test"), 0600))

	r := helm.NewRunner(helm.RunnerConfig{HelmBin: script})
	err := r.Upgrade(helm.UpgradeParams{
		ReleaseName:    "dev-alice",
		Namespace:      "dev-alice",
		ChartPath:      "/opt/chart-repo",
		ValuesFile:     "/data/envs/dev-alice/values.yaml",
		KubeconfigPath: kubeconfigFile,
		LogPath:        filepath.Join(t.TempDir(), "helm.log"),
		SecretValues:   map[string]string{"openai-api-key": "sk-test"},
	})
	require.NoError(t, err)

	raw, err := os.ReadFile(captureFile)
	require.NoError(t, err)
	capturedArgs = strings.Fields(string(raw))

	// Count --values occurrences: one for ValuesFile, one for secrets temp file
	valuesCount := 0
	for _, a := range capturedArgs {
		if a == "--values" {
			valuesCount++
		}
	}
	assert.Equal(t, 2, valuesCount, "expected two --values flags when SecretValues provided")
}
```

Also add `"strings"` to the import block in `runner_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/helm/... -run TestRunner_Upgrade_WithSecretValues -v`
Expected: FAIL — `SecretValues` field undefined on `UpgradeParams`

- [ ] **Step 3: Update UpgradeParams and Upgrade in runner.go**

Replace `internal/helm/runner.go` with:

```go
package helm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RunnerConfig struct {
	HelmBin string
}

type Runner struct {
	bin string
}

func NewRunner(cfg RunnerConfig) *Runner {
	bin := cfg.HelmBin
	if bin == "" {
		bin = "helm"
	}
	return &Runner{bin: bin}
}

type UpgradeParams struct {
	ReleaseName    string
	Namespace      string
	ChartPath      string
	ValuesFile     string
	KubeconfigPath string
	LogPath        string
	SecretValues   map[string]string
}

func (r *Runner) Upgrade(p UpgradeParams) error {
	args := []string{
		"upgrade", "--install", p.ReleaseName, p.ChartPath,
		"--namespace", p.Namespace,
		"--create-namespace",
		"--values", p.ValuesFile,
		"--kubeconfig", p.KubeconfigPath,
		"--wait", "--timeout", "10m",
	}
	if len(p.SecretValues) > 0 {
		f, err := os.CreateTemp("", "helm-secrets-*.yaml")
		if err != nil {
			return fmt.Errorf("create secrets temp file: %w", err)
		}
		defer os.Remove(f.Name())
		if err := yaml.NewEncoder(f).Encode(p.SecretValues); err != nil {
			f.Close()
			return fmt.Errorf("write secrets temp file: %w", err)
		}
		f.Close()
		args = append(args, "--values", f.Name())
	}
	return r.run(p.LogPath, args...)
}

type StatusParams struct {
	ReleaseName    string
	Namespace      string
	KubeconfigPath string
}

func (r *Runner) Status(p StatusParams) (string, error) {
	cmd := exec.Command(r.bin, "status", p.ReleaseName,
		"--namespace", p.Namespace,
		"--kubeconfig", p.KubeconfigPath,
		"--output", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("helm status: %w", err)
	}
	return string(out), nil
}

func (r *Runner) run(logPath string, args ...string) error {
	dir := filepath.Dir(logPath)
	os.MkdirAll(dir, 0755)

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(r.bin, args...)
	cmd.Stdout = io.MultiWriter(logFile, os.Stdout)
	cmd.Stderr = io.MultiWriter(logFile, os.Stderr)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm %s: %w", args[0], err)
	}
	return nil
}
```

- [ ] **Step 4: Run all helm tests to verify they pass**

Run: `go test ./internal/helm/... -v`
Expected: all tests PASS (including existing Upgrade_Success and Upgrade_Failure)

- [ ] **Step 5: Commit**

```bash
git add internal/helm/runner.go internal/helm/runner_test.go
git commit -m "feat: helm runner injects secret values via temp --values file"
```

---

### Task 6: Server wiring

**Files:**
- Modify: `internal/server/server.go`
- Modify: `cmd/portal/main.go`

- [ ] **Step 1: Update server.New to accept SecretFetcher**

In `internal/server/server.go`, update the `New` function signature and wiring:

Change the import block to add the secrets package:
```go
import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/auth"
	"github.com/emdash/kindle/internal/chartversions"
	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/db"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/helm"
	"github.com/emdash/kindle/internal/iapssh"
	"github.com/emdash/kindle/internal/k8s"
	"github.com/emdash/kindle/internal/kubeconfig"
	"github.com/emdash/kindle/internal/presets"
	"github.com/emdash/kindle/internal/secrets"
	"github.com/emdash/kindle/internal/terraform"
)
```

Change the `New` function signature from:
```go
func New(cfg *config.Config, vmClient gcp.VMClient, database *db.DB, staticFiles embed.FS) *Server {
```
to:
```go
func New(cfg *config.Config, vmClient gcp.VMClient, database *db.DB, staticFiles embed.FS, fetcher secrets.SecretFetcher) *Server {
```

Replace the `HelmUpgrade` closure inside `New` (lines 138–148) with:
```go
HelmUpgrade: func(envName, valuesPath, kubeconfigPath string) error {
    secretVals, err := fetcher.FetchSecrets(context.Background(), envName)
    if err != nil {
        slog.Error("failed to fetch secrets", "env", envName, "error", err)
        return fmt.Errorf("fetch secrets: %w", err)
    }
    return helmRunner.Upgrade(helm.UpgradeParams{
        ReleaseName:    envName,
        Namespace:      envName,
        ChartPath:      "/opt/chart-repo",
        ValuesFile:     valuesPath,
        KubeconfigPath: kubeconfigPath,
        LogPath:        filepath.Join(cfg.DataDir, "envs", envName, "ops", "helm-upgrade.log"),
        SecretValues:   secretVals,
    })
},
```

- [ ] **Step 2: Update main.go to create the fetcher**

Replace `cmd/portal/main.go` with:

```go
package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/db"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/secrets"
	"github.com/emdash/kindle/internal/server"
)

//go:embed web/dist
var staticFiles embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	os.MkdirAll(cfg.DataDir, 0755)
	database, err := db.Open(filepath.Join(cfg.DataDir, "portal.db"))
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	ctx := context.Background()
	vmClient, err := gcp.NewVMClient(ctx, cfg.GCPProjectID)
	if err != nil {
		slog.Error("failed to create GCP client", "error", err)
		os.Exit(1)
	}

	fetcher, err := secrets.NewFetcherFromConfig()
	if err != nil {
		slog.Error("failed to initialize secret fetcher", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, vmClient, database, staticFiles, fetcher)
	slog.Info("portal listening", "addr", srv.Addr())
	if err := http.ListenAndServe(srv.Addr(), srv.Handler()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Verify the build compiles**

Run: `go build ./...`
Expected: exits 0, no errors

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go cmd/portal/main.go
git commit -m "feat: wire SecretFetcher into server and HelmUpgrade closure"
```
