# Infrastructure Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GCP Compute client, preset template loader, chart version lister, Terraform runner with in-memory job map, and env list/create/delete API endpoints. Also write the Terraform module that provisions env VMs.

**Architecture:** Each external dependency (GCP, Terraform, git) is behind an interface so handlers can be tested without real infrastructure. Terraform is shelled out as a subprocess; state lives locally under `data/envs/<name>/terraform/`. One operation per env at a time enforced by the job map.

**Prerequisite:** Plan 1 (Backend Foundation) must be complete.

**Tech Stack:** `cloud.google.com/go/compute`, `google.golang.org/api`, `gopkg.in/yaml.v3`, Terraform CLI (external), `os/exec`

---

## File Map

| File | Responsibility |
|------|---------------|
| `internal/gcp/compute.go` | `VMClient` interface + GCP Compute API implementation |
| `internal/gcp/compute_test.go` | Tests using a fake VMClient |
| `internal/presets/schema.go` | `Preset` and `UserEditableField` types |
| `internal/presets/loader.go` | Load presets from `./presets/` directory |
| `internal/presets/loader_test.go` | Tests with fixture presets |
| `internal/chartversions/lister.go` | `git ls-remote` wrapper, returns tags + branches |
| `internal/chartversions/lister_test.go` | Tests with a local bare git repo fixture |
| `internal/terraform/jobs.go` | In-memory job map with mutex |
| `internal/terraform/jobs_test.go` | Concurrent access tests |
| `internal/terraform/runner.go` | Shell-out runner: init, apply, destroy |
| `internal/terraform/runner_test.go` | Tests using fake terraform script |
| `internal/api/envs.go` | `GET /api/envs`, `GET /api/envs/{name}`, `POST /api/envs`, `DELETE /api/envs/{name}` |
| `internal/api/envs_test.go` | Handler tests with fake VMClient + fake Runner |
| `internal/api/templates.go` | `GET /api/templates`, `GET /api/templates/{name}/chart-versions` |
| `internal/api/templates_test.go` | Handler tests with fixture presets |
| `internal/server/server.go` | Register new routes (modify existing) |
| `terraform/module/variables.tf` | Input variables for the Terraform module |
| `terraform/module/main.tf` | GCE VM, static IP, firewall, Cloud DNS A record |
| `terraform/module/outputs.tf` | Output: VM name, external IP |
| `terraform/module/startup.sh` | k3s install + Helm chart deploy script |
| `presets/small-dev/template.yaml` | Example preset definition |
| `presets/small-dev/values.yaml` | Baseline Helm values for small-dev preset |

---

## Task 1: GCP Compute client

**Files:**
- Create: `internal/gcp/compute.go`
- Create: `internal/gcp/compute_test.go`

- [ ] **Step 1: Add GCP dependency**

```bash
go get cloud.google.com/go/compute/apiv1@latest
go get google.golang.org/api@latest
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/gcp/compute_test.go
package gcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/emdash/kindle/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVMClient implements gcp.VMClient for tests.
type fakeVMClient struct {
	vms []*gcp.VMInfo
}

func (f *fakeVMClient) ListEnvVMs(ctx context.Context) ([]*gcp.VMInfo, error) {
	return f.vms, nil
}

func (f *fakeVMClient) GetVM(ctx context.Context, name string) (*gcp.VMInfo, error) {
	for _, vm := range f.vms {
		if vm.Name == name {
			return vm, nil
		}
	}
	return nil, gcp.ErrVMNotFound
}

func TestFakeClient_ListEnvVMs(t *testing.T) {
	client := &fakeVMClient{
		vms: []*gcp.VMInfo{
			{Name: "dev-alice", Owner: "alice@example.com", MachineType: "e2-standard-2", Zone: "us-central1-a", ExternalIP: "1.2.3.4", CreatedAt: time.Now()},
		},
	}
	vms, err := client.ListEnvVMs(context.Background())
	require.NoError(t, err)
	assert.Len(t, vms, 1)
	assert.Equal(t, "dev-alice", vms[0].Name)
}

func TestFakeClient_GetVM_NotFound(t *testing.T) {
	client := &fakeVMClient{vms: nil}
	_, err := client.GetVM(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, gcp.ErrVMNotFound)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/gcp/... -v
```

Expected: FAIL — `gcp` package not found.

- [ ] **Step 4: Write `internal/gcp/compute.go`**

```go
package gcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
)

var ErrVMNotFound = errors.New("VM not found")

type VMInfo struct {
	Name        string
	Owner       string
	Template    string
	MachineType string
	Zone        string
	ExternalIP  string
	InternalIP  string
	DiskSizeGB  int64
	CreatedAt   time.Time
	Status      string // GCP status: RUNNING, TERMINATED, etc.
}

type VMClient interface {
	ListEnvVMs(ctx context.Context) ([]*VMInfo, error)
	GetVM(ctx context.Context, name string) (*VMInfo, error)
}

type gcpVMClient struct {
	project string
	client  *compute.InstancesClient
}

func NewVMClient(ctx context.Context, project string) (VMClient, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create compute client: %w", err)
	}
	return &gcpVMClient{project: project, client: c}, nil
}

func (c *gcpVMClient) ListEnvVMs(ctx context.Context) ([]*VMInfo, error) {
	req := &computepb.AggregatedListInstancesRequest{
		Project: c.project,
		Filter:  strPtr("labels.managed-by=env-portal"),
	}
	var vms []*VMInfo
	it := c.client.AggregatedList(ctx, req)
	for {
		pair, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, inst := range pair.Value.Instances {
			vms = append(vms, instanceToVMInfo(inst))
		}
	}
	return vms, nil
}

func (c *gcpVMClient) GetVM(ctx context.Context, name string) (*VMInfo, error) {
	vms, err := c.ListEnvVMs(ctx)
	if err != nil {
		return nil, err
	}
	for _, vm := range vms {
		if vm.Name == name {
			return vm, nil
		}
	}
	return nil, ErrVMNotFound
}

func instanceToVMInfo(inst *computepb.Instance) *VMInfo {
	info := &VMInfo{
		Name:        inst.GetName(),
		MachineType: lastSegment(inst.GetMachineType()),
		Zone:        lastSegment(inst.GetZone()),
		Status:      inst.GetStatus(),
	}
	if inst.Labels != nil {
		info.Owner = inst.Labels["owner"]
		info.Template = inst.Labels["template"]
	}
	for _, ni := range inst.NetworkInterfaces {
		info.InternalIP = ni.GetNetworkIP()
		for _, ac := range ni.AccessConfigs {
			info.ExternalIP = ac.GetNatIP()
		}
	}
	for _, disk := range inst.Disks {
		if disk.GetBoot() {
			info.DiskSizeGB = disk.GetDiskSizeGb()
		}
	}
	t, _ := time.Parse(time.RFC3339, inst.GetCreationTimestamp())
	info.CreatedAt = t
	return info
}

func lastSegment(s string) string {
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/gcp/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/ go.mod go.sum
git commit -m "feat: add GCP Compute VMClient interface and implementation"
```

---

## Task 2: Preset loader

**Files:**
- Create: `internal/presets/schema.go`
- Create: `internal/presets/loader.go`
- Create: `internal/presets/loader_test.go`
- Create: `presets/small-dev/template.yaml`
- Create: `presets/small-dev/values.yaml`

- [ ] **Step 1: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3@v3.0.1
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/presets/loader_test.go
package presets_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/emdash/kindle/internal/presets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPresetsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestLoader_Load(t *testing.T) {
	loader := presets.NewLoader(testPresetsDir())
	list, err := loader.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "small-dev", list[0].Name)
	assert.Equal(t, "e2-standard-2", list[0].Defaults.MachineType)
	assert.Equal(t, int64(50), list[0].Defaults.DiskSizeGB)
	assert.Len(t, list[0].UserEditable, 3)
}

func TestLoader_Get_NotFound(t *testing.T) {
	loader := presets.NewLoader(testPresetsDir())
	_, err := loader.Get("nonexistent")
	assert.ErrorIs(t, err, presets.ErrPresetNotFound)
}

func TestLoader_BaseValues(t *testing.T) {
	loader := presets.NewLoader(testPresetsDir())
	preset, err := loader.Get("small-dev")
	require.NoError(t, err)
	vals, err := preset.BaseValues()
	require.NoError(t, err)
	assert.NotEmpty(t, vals)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/presets/... -v
```

Expected: FAIL — `presets` package not found.

- [ ] **Step 4: Create test fixture at `internal/presets/testdata/small-dev/template.yaml`**

```bash
mkdir -p internal/presets/testdata/small-dev
```

```yaml
# internal/presets/testdata/small-dev/template.yaml
name: small-dev
description: "Lightweight dev environment"
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

```yaml
# internal/presets/testdata/small-dev/values.yaml
api:
  image:
    tag: latest
  replicas: 1
worker:
  image:
    tag: latest
```

- [ ] **Step 5: Write `internal/presets/schema.go`**

```go
package presets

type Preset struct {
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	Defaults     PresetDefaults    `yaml:"defaults"`
	ImageTagKeys map[string]string `yaml:"image_tag_keys"`
	UserEditable []UserEditableField `yaml:"user_editable"`
	dir          string
}

type PresetDefaults struct {
	MachineType string `yaml:"machine_type"`
	DiskSizeGB  int64  `yaml:"disk_size_gb"`
	Zone        string `yaml:"zone"`
	ChartRef    string `yaml:"chart_ref"`
}

type UserEditableField struct {
	Key     string   `yaml:"key"`
	Label   string   `yaml:"label"`
	Type    string   `yaml:"type"` // enum | integer | string | boolean
	Options []string `yaml:"options,omitempty"`
	Min     *int64   `yaml:"min,omitempty"`
	Max     *int64   `yaml:"max,omitempty"`
	Default any      `yaml:"default,omitempty"`
}
```

- [ ] **Step 6: Write `internal/presets/loader.go`**

```go
package presets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var ErrPresetNotFound = errors.New("preset not found")

type Loader struct {
	dir string
}

func NewLoader(dir string) *Loader {
	return &Loader{dir: dir}
}

func (l *Loader) List() ([]*Preset, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("read presets dir: %w", err)
	}
	var presets []*Preset
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := l.load(e.Name())
		if err != nil {
			return nil, fmt.Errorf("load preset %s: %w", e.Name(), err)
		}
		presets = append(presets, p)
	}
	return presets, nil
}

func (l *Loader) Get(name string) (*Preset, error) {
	p, err := l.load(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrPresetNotFound
	}
	return p, err
}

func (l *Loader) load(name string) (*Preset, error) {
	path := filepath.Join(l.dir, name, "template.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Preset
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	p.dir = filepath.Join(l.dir, name)
	return &p, nil
}

func (p *Preset) BaseValues() ([]byte, error) {
	return os.ReadFile(filepath.Join(p.dir, "values.yaml"))
}
```

- [ ] **Step 7: Create real presets in `presets/` directory**

```bash
mkdir -p presets/small-dev
```

```yaml
# presets/small-dev/template.yaml
name: small-dev
description: "Lightweight dev environment"
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

```yaml
# presets/small-dev/values.yaml
api:
  image:
    tag: latest
  replicas: 1
worker:
  image:
    tag: latest
```

- [ ] **Step 8: Run tests to verify they pass**

```bash
go test ./internal/presets/... -v
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/presets/ presets/ go.mod go.sum
git commit -m "feat: add preset loader with YAML schema"
```

---

## Task 3: Chart versions lister

**Files:**
- Create: `internal/chartversions/lister.go`
- Create: `internal/chartversions/lister_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/chartversions/lister_test.go
package chartversions_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/emdash/kindle/internal/chartversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createLocalBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "chart-repo.git")

	// Init bare repo with a tag and branch
	cmds := [][]string{
		{"git", "init", "--bare", repoDir},
		{"git", "-C", dir, "init", "work"},
		{"git", "-C", filepath.Join(dir, "work"), "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", filepath.Join(dir, "work"), "tag", "v1.0.0"},
		{"git", "-C", filepath.Join(dir, "work"), "checkout", "-b", "staging"},
		{"git", "-C", filepath.Join(dir, "work"), "commit", "--allow-empty", "-m", "staging"},
		{"git", "-C", filepath.Join(dir, "work"), "remote", "add", "origin", repoDir},
		{"git", "-C", filepath.Join(dir, "work"), "push", "origin", "--all"},
		{"git", "-C", filepath.Join(dir, "work"), "push", "origin", "--tags"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd: %v, output: %s", args, out)
	}
	return repoDir
}

func TestLister_List(t *testing.T) {
	repoURL := createLocalBareRepo(t)
	lister := chartversions.NewLister(repoURL)

	versions, err := lister.List()
	require.NoError(t, err)

	var names []string
	for _, v := range versions {
		names = append(names, v.Ref)
	}
	assert.Contains(t, names, "refs/tags/v1.0.0")
	assert.Contains(t, names, "refs/heads/staging")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/chartversions/... -v
```

Expected: FAIL — `chartversions` package not found.

- [ ] **Step 3: Write `internal/chartversions/lister.go`**

```go
package chartversions

import (
	"fmt"
	"os/exec"
	"strings"
)

type Version struct {
	Ref  string // e.g. refs/tags/v1.0.0 or refs/heads/main
	SHA  string
	Kind string // "tag" or "branch"
}

type Lister struct {
	repoURL string
}

func NewLister(repoURL string) *Lister {
	return &Lister{repoURL: repoURL}
}

func (l *Lister) List() ([]Version, error) {
	out, err := exec.Command("git", "ls-remote", "--heads", "--tags", l.repoURL).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote: %w", err)
	}
	return parseRefs(string(out)), nil
}

func parseRefs(output string) []Version {
	var versions []Version
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		sha, ref := parts[0], parts[1]
		kind := "branch"
		if strings.HasPrefix(ref, "refs/tags/") {
			kind = "tag"
		}
		// Skip tag dereference lines (ending in ^{})
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		versions = append(versions, Version{Ref: ref, SHA: sha, Kind: kind})
	}
	return versions
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/chartversions/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/chartversions/
git commit -m "feat: add chart versions lister using git ls-remote"
```

---

## Task 4: Terraform job map

**Files:**
- Create: `internal/terraform/jobs.go`
- Create: `internal/terraform/jobs_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/terraform/jobs_test.go
package terraform_test

import (
	"sync"
	"testing"
	"time"

	"github.com/emdash/kindle/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobMap_SetAndGet(t *testing.T) {
	jm := terraform.NewJobMap()
	jm.Set("env-alice", terraform.JobStatus{State: terraform.StateProvisioning, Actor: "alice@example.com"})

	job, ok := jm.Get("env-alice")
	require.True(t, ok)
	assert.Equal(t, terraform.StateProvisioning, job.State)
	assert.Equal(t, "alice@example.com", job.Actor)
}

func TestJobMap_Delete(t *testing.T) {
	jm := terraform.NewJobMap()
	jm.Set("env-bob", terraform.JobStatus{State: terraform.StateProvisioning})
	jm.Delete("env-bob")
	_, ok := jm.Get("env-bob")
	assert.False(t, ok)
}

func TestJobMap_ConcurrentAccess(t *testing.T) {
	jm := terraform.NewJobMap()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("env-%d", i)
			jm.Set(name, terraform.JobStatus{State: terraform.StateProvisioning, StartedAt: time.Now()})
			jm.Get(name)
			jm.Delete(name)
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/terraform/... -v
```

Expected: FAIL — `terraform` package not found.

- [ ] **Step 3: Write `internal/terraform/jobs.go`**

```go
package terraform

import (
	"sync"
	"time"
)

const (
	StateProvisioning = "provisioning"
	StateDeleting     = "deleting"
	StateIdle         = "idle"
)

type JobStatus struct {
	State     string
	StartedAt time.Time
	Actor     string
	LogPath   string
	Err       error
}

type JobMap struct {
	mu   sync.RWMutex
	jobs map[string]JobStatus
}

func NewJobMap() *JobMap {
	return &JobMap{jobs: make(map[string]JobStatus)}
}

func (m *JobMap) Set(name string, status JobStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[name] = status
}

func (m *JobMap) Get(name string) (JobStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.jobs[name]
	return s, ok
}

func (m *JobMap) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, name)
}

func (m *JobMap) InFlight(name string) bool {
	s, ok := m.Get(name)
	if !ok {
		return false
	}
	return s.State == StateProvisioning || s.State == StateDeleting
}
```

- [ ] **Step 4: Add missing import to test file**

```go
// Add at top of internal/terraform/jobs_test.go
import "fmt"
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/terraform/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/terraform/jobs.go internal/terraform/jobs_test.go
git commit -m "feat: add in-memory terraform job map with concurrent access"
```

---

## Task 5: Terraform runner

**Files:**
- Create: `internal/terraform/runner.go`
- Create: `internal/terraform/runner_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/terraform/runner_test.go
package terraform_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emdash/kindle/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Apply_WritesLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "apply.log")

	// Create a fake terraform script that exits 0
	fakeScript := filepath.Join(dir, "terraform")
	require.NoError(t, os.WriteFile(fakeScript, []byte("#!/bin/sh\necho 'apply complete'\n"), 0755))

	runner := terraform.NewRunner(terraform.RunnerConfig{
		TerraformBin: fakeScript,
	})

	err := runner.Apply(dir, logPath)
	require.NoError(t, err)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(logContent), "apply complete")
}

func TestRunner_Apply_FailsOnNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "apply.log")

	fakeScript := filepath.Join(dir, "terraform")
	require.NoError(t, os.WriteFile(fakeScript, []byte("#!/bin/sh\necho 'error' >&2\nexit 1\n"), 0755))

	runner := terraform.NewRunner(terraform.RunnerConfig{
		TerraformBin: fakeScript,
	})

	err := runner.Apply(dir, logPath)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/terraform/... -run TestRunner -v
```

Expected: FAIL — `Runner` not defined.

- [ ] **Step 3: Write `internal/terraform/runner.go`**

```go
package terraform

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type RunnerConfig struct {
	TerraformBin string // defaults to "terraform" in PATH
}

type Runner struct {
	bin string
}

func NewRunner(cfg RunnerConfig) *Runner {
	bin := cfg.TerraformBin
	if bin == "" {
		bin = "terraform"
	}
	return &Runner{bin: bin}
}

func (r *Runner) Apply(workDir, logPath string) error {
	return r.run(workDir, logPath, "apply", "-auto-approve", "-input=false")
}

func (r *Runner) Destroy(workDir, logPath string) error {
	return r.run(workDir, logPath, "destroy", "-auto-approve", "-input=false")
}

func (r *Runner) Init(workDir, logPath string) error {
	return r.run(workDir, logPath, "init", "-input=false")
}

func (r *Runner) run(workDir, logPath string, args ...string) error {
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(r.bin, args...)
	cmd.Dir = workDir
	cmd.Stdout = io.MultiWriter(logFile, os.Stdout)
	cmd.Stderr = io.MultiWriter(logFile, os.Stderr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform %s: %w", args[0], err)
	}
	return nil
}
```

- [ ] **Step 4: Run all terraform tests**

```bash
go test ./internal/terraform/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/terraform/runner.go internal/terraform/runner_test.go
git commit -m "feat: add terraform runner that shells out to terraform CLI"
```

---

## Task 6: Env API handlers (list, detail, create, delete)

**Files:**
- Create: `internal/api/envs.go`
- Create: `internal/api/envs_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/envs_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeVMClient struct {
	vms []*gcp.VMInfo
}

func (f *fakeVMClient) ListEnvVMs(ctx context.Context) ([]*gcp.VMInfo, error) {
	return f.vms, nil
}
func (f *fakeVMClient) GetVM(ctx context.Context, name string) (*gcp.VMInfo, error) {
	for _, vm := range f.vms {
		if vm.Name == name {
			return vm, nil
		}
	}
	return nil, gcp.ErrVMNotFound
}

func newTestEnvHandler() *api.EnvsHandler {
	return api.NewEnvsHandler(api.EnvsHandlerDeps{
		VMClient: &fakeVMClient{
			vms: []*gcp.VMInfo{
				{Name: "dev-alice", Owner: "alice@example.com", Template: "small-dev", MachineType: "e2-standard-2", Zone: "us-central1-a", ExternalIP: "1.2.3.4", CreatedAt: time.Now()},
			},
		},
		Jobs: terraform.NewJobMap(),
	})
}

func TestEnvsHandler_List(t *testing.T) {
	h := newTestEnvHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/envs", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body, 1)
	assert.Equal(t, "dev-alice", body[0]["name"])
}

func TestEnvsHandler_Get_NotFound(t *testing.T) {
	h := newTestEnvHandler()
	r := chi.NewRouter()
	r.Get("/api/envs/{name}", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/envs/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEnvsHandler_Create_DuplicateName(t *testing.T) {
	h := newTestEnvHandler()
	body := map[string]any{"name": "dev-alice", "template": "small-dev"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/envs", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -v
```

Expected: FAIL — `api` package not found.

- [ ] **Step 3: Write `internal/api/envs.go`**

```go
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/terraform"
)

type EnvsHandlerDeps struct {
	VMClient   gcp.VMClient
	Jobs       *terraform.JobMap
	TFRunner   *terraform.Runner
	ModuleDir  string // path to terraform/module/
	DataDir    string
	ZoneDomain string
}

type EnvsHandler struct {
	deps EnvsHandlerDeps
}

func NewEnvsHandler(deps EnvsHandlerDeps) *EnvsHandler {
	return &EnvsHandler{deps: deps}
}

type envSummary struct {
	Name        string    `json:"name"`
	Owner       string    `json:"owner"`
	Template    string    `json:"template"`
	MachineType string    `json:"machine_type"`
	Zone        string    `json:"zone"`
	ExternalIP  string    `json:"external_ip"`
	CreatedAt   time.Time `json:"created_at"`
	JobState    string    `json:"job_state,omitempty"`
}

func (h *EnvsHandler) List(w http.ResponseWriter, r *http.Request) {
	vms, err := h.deps.VMClient.ListEnvVMs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]envSummary, len(vms))
	for i, vm := range vms {
		result[i] = vmToSummary(vm)
		if job, ok := h.deps.Jobs.Get(vm.Name); ok {
			result[i].JobState = job.State
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *EnvsHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	vm, err := h.deps.VMClient.GetVM(r.Context(), name)
	if errors.Is(err, gcp.ErrVMNotFound) {
		writeError(w, http.StatusNotFound, "env not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summary := vmToSummary(vm)
	if job, ok := h.deps.Jobs.Get(name); ok {
		summary.JobState = job.State
	}
	writeJSON(w, http.StatusOK, summary)
}

type createEnvRequest struct {
	Name     string         `json:"name"`
	Template string         `json:"template"`
	Overrides map[string]any `json:"overrides"`
}

func (h *EnvsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createEnvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Template == "" {
		writeError(w, http.StatusBadRequest, "name and template are required")
		return
	}

	// Check uniqueness
	_, err := h.deps.VMClient.GetVM(r.Context(), req.Name)
	if err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("env %s already exists", req.Name))
		return
	}
	if !errors.Is(err, gcp.ErrVMNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check no in-flight op
	if h.deps.Jobs.InFlight(req.Name) {
		writeError(w, http.StatusConflict, "operation already in progress for this env")
		return
	}

	actor := actorFromContext(r)
	logPath := h.logPath(req.Name, "create")
	workDir := filepath.Join(h.deps.DataDir, "envs", req.Name, "terraform")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create work dir")
		return
	}

	h.deps.Jobs.Set(req.Name, terraform.JobStatus{
		State:     terraform.StateProvisioning,
		StartedAt: time.Now(),
		Actor:     actor,
		LogPath:   logPath,
	})

	go h.runApply(req.Name, workDir, logPath, actor)

	writeJSON(w, http.StatusAccepted, map[string]string{"name": req.Name, "status": "provisioning"})
}

func (h *EnvsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	_, err := h.deps.VMClient.GetVM(r.Context(), name)
	if errors.Is(err, gcp.ErrVMNotFound) {
		writeError(w, http.StatusNotFound, "env not found")
		return
	}
	if h.deps.Jobs.InFlight(name) {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	actor := actorFromContext(r)
	logPath := h.logPath(name, "delete")
	workDir := filepath.Join(h.deps.DataDir, "envs", name, "terraform")

	h.deps.Jobs.Set(name, terraform.JobStatus{
		State:     terraform.StateDeleting,
		StartedAt: time.Now(),
		Actor:     actor,
		LogPath:   logPath,
	})

	go h.runDestroy(name, workDir, logPath)

	writeJSON(w, http.StatusAccepted, map[string]string{"name": name, "status": "deleting"})
}

func (h *EnvsHandler) runApply(name, workDir, logPath, actor string) {
	if err := h.deps.TFRunner.Init(workDir, logPath); err == nil {
		h.deps.TFRunner.Apply(workDir, logPath)
	}
	h.deps.Jobs.Delete(name)
}

func (h *EnvsHandler) runDestroy(name, workDir, logPath string) {
	h.deps.TFRunner.Destroy(workDir, logPath)
	h.deps.Jobs.Delete(name)
	// Move to archive
	src := filepath.Join(h.deps.DataDir, "envs", name)
	dst := filepath.Join(h.deps.DataDir, "archive", name)
	os.MkdirAll(filepath.Dir(dst), 0755)
	os.Rename(src, dst)
}

func (h *EnvsHandler) logPath(name, action string) string {
	ts := time.Now().Format("20060102-150405")
	return filepath.Join(h.deps.DataDir, "envs", name, "ops", fmt.Sprintf("%s-%s.log", ts, action))
}

func vmToSummary(vm *gcp.VMInfo) envSummary {
	return envSummary{
		Name:        vm.Name,
		Owner:       vm.Owner,
		Template:    vm.Template,
		MachineType: vm.MachineType,
		Zone:        vm.Zone,
		ExternalIP:  vm.ExternalIP,
		CreatedAt:   vm.CreatedAt,
	}
}

func actorFromContext(r *http.Request) string {
	// Will be populated by session middleware in Plan 3.
	// For now return empty string.
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/api/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/envs.go internal/api/envs_test.go
git commit -m "feat: add env list/get/create/delete API handlers"
```

---

## Task 7: Templates API handlers

**Files:**
- Create: `internal/api/templates.go`
- Create: `internal/api/templates_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/templates_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/presets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPresetsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "presets", "testdata")
}

func TestTemplatesHandler_List(t *testing.T) {
	loader := presets.NewLoader(filepath.Join(filepath.Dir(testPresetsDir()), "testdata"))
	h := api.NewTemplatesHandler(api.TemplatesHandlerDeps{
		Loader: loader,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body, 1)
	assert.Equal(t, "small-dev", body[0]["name"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -run TestTemplates -v
```

Expected: FAIL — `TemplatesHandler` not defined.

- [ ] **Step 3: Write `internal/api/templates.go`**

```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/chartversions"
	"github.com/emdash/kindle/internal/presets"
)

type TemplatesHandlerDeps struct {
	Loader  *presets.Loader
	Lister  *chartversions.Lister
}

type TemplatesHandler struct {
	deps TemplatesHandlerDeps
}

func NewTemplatesHandler(deps TemplatesHandlerDeps) *TemplatesHandler {
	return &TemplatesHandler{deps: deps}
}

func (h *TemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.deps.Loader.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type presetSummary struct {
		Name         string                  `json:"name"`
		Description  string                  `json:"description"`
		Defaults     any                     `json:"defaults"`
		UserEditable []presets.UserEditableField `json:"user_editable"`
	}
	result := make([]presetSummary, len(list))
	for i, p := range list {
		result[i] = presetSummary{
			Name:         p.Name,
			Description:  p.Description,
			Defaults:     p.Defaults,
			UserEditable: p.UserEditable,
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *TemplatesHandler) ChartVersions(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "name") // preset name — not used in v1, same chart for all presets
	if h.deps.Lister == nil {
		writeError(w, http.StatusInternalServerError, "chart versions lister not configured")
		return
	}
	versions, err := h.deps.Lister.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/api/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/templates.go internal/api/templates_test.go
git commit -m "feat: add templates list and chart versions API handlers"
```

---

## Task 8: Register new routes in server

**Files:**
- Modify: `internal/server/server.go`

- [ ] **Step 1: Update `internal/server/server.go` to register env and template routes**

```go
package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/auth"
	"github.com/emdash/kindle/internal/chartversions"
	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/presets"
	"github.com/emdash/kindle/internal/terraform"
)

type Server struct {
	cfg    *config.Config
	auth   *auth.Handler
	router chi.Router
}

func New(cfg *config.Config, vmClient gcp.VMClient) *Server {
	authHandler := auth.NewHandler(auth.HandlerConfig{
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		RedirectURL:     fmt.Sprintf("http://localhost:%s/auth/callback", cfg.Port),
		CorporateDomain: cfg.CorporateDomain,
		SessionSecret:   cfg.SessionSecret,
	})

	jobs := terraform.NewJobMap()
	tfRunner := terraform.NewRunner(terraform.RunnerConfig{})

	presetLoader := presets.NewLoader(cfg.PresetsDir)
	chartLister := chartversions.NewLister(cfg.ChartRepoURL)

	envsHandler := api.NewEnvsHandler(api.EnvsHandlerDeps{
		VMClient:   vmClient,
		Jobs:       jobs,
		TFRunner:   tfRunner,
		ModuleDir:  "terraform/module",
		DataDir:    cfg.DataDir,
		ZoneDomain: cfg.ZoneDomain,
	})

	templatesHandler := api.NewTemplatesHandler(api.TemplatesHandlerDeps{
		Loader: presetLoader,
		Lister: chartLister,
	})

	s := &Server{cfg: cfg, auth: authHandler}
	s.router = s.buildRouter(envsHandler, templatesHandler)
	return s
}

func (s *Server) buildRouter(envs *api.EnvsHandler, templates *api.TemplatesHandler) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", HealthHandler)
	r.Get("/auth/login", s.auth.LoginHandler)
	r.Get("/auth/callback", s.auth.CallbackHandler)
	r.Get("/auth/logout", s.auth.LogoutHandler)

	r.Group(func(r chi.Router) {
		r.Use(s.auth.Middleware)
		r.Get("/api/envs", envs.List)
		r.Post("/api/envs", envs.Create)
		r.Get("/api/envs/{name}", envs.Get)
		r.Delete("/api/envs/{name}", envs.Delete)
		r.Get("/api/templates", templates.List)
		r.Get("/api/templates/{name}/chart-versions", templates.ChartVersions)
	})

	return r
}

func (s *Server) Handler() http.Handler { return s.router }
func (s *Server) Addr() string          { return ":" + s.cfg.Port }
```

- [ ] **Step 2: Update `cmd/portal/main.go` to pass vmClient**

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	vmClient, err := gcp.NewVMClient(ctx, cfg.GCPProjectID)
	if err != nil {
		slog.Error("failed to create GCP client", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, vmClient)
	slog.Info("portal listening", "addr", srv.Addr())
	if err := http.ListenAndServe(srv.Addr(), srv.Handler()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build and test**

```bash
make build && make test
```

Expected: builds and all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/server/server.go cmd/portal/main.go
git commit -m "feat: register env and templates routes in server"
```

---

## Task 9: Terraform module

**Files:**
- Create: `terraform/module/variables.tf`
- Create: `terraform/module/main.tf`
- Create: `terraform/module/outputs.tf`
- Create: `terraform/module/startup.sh`

- [ ] **Step 1: Create `terraform/module/variables.tf`**

```bash
mkdir -p terraform/module
```

```hcl
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
  type        = string
  default     = "e2-standard-2"
}

variable "disk_size_gb" {
  type        = number
  default     = 50
}

variable "zone" {
  type        = string
  default     = "us-central1-a"
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
```

- [ ] **Step 2: Create `terraform/module/main.tf`**

```hcl
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
```

- [ ] **Step 3: Create `terraform/module/outputs.tf`**

```hcl
output "external_ip" {
  value = google_compute_address.env.address
}

output "vm_name" {
  value = google_compute_instance.env.name
}

output "fqdn" {
  value = "${var.env_name}.${var.zone_domain}"
}
```

- [ ] **Step 4: Create `terraform/module/startup.sh`**

```bash
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
curl -sfL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Clone chart repo at specified ref
git clone --depth 1 --branch "$CHART_REF" "$CHART_REPO_URL" /opt/chart-repo

# Deploy Helm chart
helm upgrade --install "$ENV_NAME" /opt/chart-repo \
  --namespace "$ENV_NAME" \
  --create-namespace \
  --values /opt/portal-values.yaml \
  --kubeconfig /etc/rancher/k3s/k3s.yaml \
  --wait --timeout 10m
```

- [ ] **Step 5: Commit**

```bash
git add terraform/
git commit -m "feat: add Terraform module for env VM provisioning"
```
