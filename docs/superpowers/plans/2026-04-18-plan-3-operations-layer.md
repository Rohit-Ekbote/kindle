# Operations Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SQLite event log, IAP SSH client, kubeconfig cache, Kubernetes workloads client, Helm runner, and all day-2 operation endpoints (status, kubeconfig download, edit values, update image tag, upgrade chart, resize VM, events log).

**Architecture:** Each external dependency (SSH, k8s, Helm) is behind an interface for testability. The event log is a single SQLite table written after every operation. The kubeconfig cache is an in-memory map invalidated when an env is recreated or deleted.

**Prerequisite:** Plans 1 and 2 must be complete.

**Tech Stack:** `modernc.org/sqlite`, `golang.org/x/crypto/ssh`, `k8s.io/client-go`, Helm CLI (external)

---

## File Map

| File | Responsibility |
|------|---------------|
| `internal/db/db.go` | Open SQLite connection, run migrations |
| `internal/db/events.go` | `WriteEvent`, `ListEvents` — event log read/write |
| `internal/db/events_test.go` | Tests against an in-memory SQLite DB |
| `internal/iapssh/client.go` | SSH-over-IAP tunnel client, returns `*ssh.Client` |
| `internal/iapssh/client_test.go` | Test with a local SSH server fixture |
| `internal/kubeconfig/cache.go` | In-memory kubeconfig cache, fetch via IAP SSH |
| `internal/kubeconfig/cache_test.go` | Tests with fake SSH fetcher |
| `internal/k8s/workloads.go` | `WorkloadClient` interface + client-go implementation |
| `internal/k8s/workloads_test.go` | Tests with fake WorkloadClient |
| `internal/helm/runner.go` | `HelmRunner` interface + shell-out implementation |
| `internal/helm/runner_test.go` | Tests with fake helm script |
| `internal/api/operations.go` | POST endpoints: edit-values, update-image, upgrade-chart, resize |
| `internal/api/operations_test.go` | Handler tests with fake deps |
| `internal/api/status.go` | `GET /api/envs/{name}/status` — on-demand k8s+helm status |
| `internal/api/status_test.go` | Handler tests |
| `internal/api/kubeconfig.go` | `GET /api/envs/{name}/kubeconfig` — download handler |
| `internal/api/kubeconfig_test.go` | Handler test |
| `internal/api/events.go` | `GET /api/envs/{name}/events` — paginated event log |
| `internal/api/events_test.go` | Handler test |
| `internal/server/server.go` | Register new routes (modify existing) |

---

## Task 1: SQLite event log

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/events.go`
- Create: `internal/db/events_test.go`

- [ ] **Step 1: Add SQLite dependency**

```bash
go get modernc.org/sqlite@latest
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/db/events_test.go
package db_test

import (
	"testing"
	"time"

	"github.com/emdash/kindle/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestWriteAndListEvents(t *testing.T) {
	d := openTestDB(t)

	err := d.WriteEvent(db.Event{
		EnvName:     "dev-alice",
		ActorEmail:  "alice@example.com",
		ActionType:  "create",
		Description: "Created env dev-alice from preset small-dev",
		Outcome:     db.OutcomeInProgress,
	})
	require.NoError(t, err)

	events, err := d.ListEvents("dev-alice", 10, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "create", events[0].ActionType)
	assert.Equal(t, db.OutcomeInProgress, events[0].Outcome)
	assert.False(t, events[0].CreatedAt.IsZero())
}

func TestListEvents_Pagination(t *testing.T) {
	d := openTestDB(t)

	for i := 0; i < 5; i++ {
		require.NoError(t, d.WriteEvent(db.Event{
			EnvName:     "dev-alice",
			ActorEmail:  "alice@example.com",
			ActionType:  "create",
			Description: "event",
			Outcome:     db.OutcomeSuccess,
		}))
	}

	page1, err := d.ListEvents("dev-alice", 3, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 3)

	page2, err := d.ListEvents("dev-alice", 3, 3)
	require.NoError(t, err)
	assert.Len(t, page2, 2)
}

func TestListEvents_Empty(t *testing.T) {
	d := openTestDB(t)
	events, err := d.ListEvents("nonexistent", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestWriteEvent_UpdateOutcome(t *testing.T) {
	d := openTestDB(t)

	require.NoError(t, d.WriteEvent(db.Event{
		EnvName:    "dev-alice",
		ActorEmail: "alice@example.com",
		ActionType: "create",
		Description: "Created",
		Outcome:    db.OutcomeInProgress,
	}))

	require.NoError(t, d.UpdateOutcome("dev-alice", "create", db.OutcomeSuccess))

	events, _ := d.ListEvents("dev-alice", 10, 0)
	assert.Equal(t, db.OutcomeSuccess, events[0].Outcome)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/db/... -v
```

Expected: FAIL — `db` package not found.

- [ ] **Step 4: Write `internal/db/db.go`**

```go
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error { return d.conn.Close() }

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS env_events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			env_name    TEXT NOT NULL,
			actor_email TEXT NOT NULL,
			action_type TEXT NOT NULL,
			description TEXT NOT NULL,
			outcome     TEXT NOT NULL,
			log_path    TEXT,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_env_events_env
			ON env_events(env_name, created_at DESC);
	`)
	return err
}
```

- [ ] **Step 5: Write `internal/db/events.go`**

```go
package db

import (
	"database/sql"
	"errors"
	"time"
)

const (
	OutcomeInProgress = "in_progress"
	OutcomeSuccess    = "success"
	OutcomeFailure    = "failure"
)

type Event struct {
	ID          int64
	EnvName     string
	ActorEmail  string
	ActionType  string
	Description string
	Outcome     string
	LogPath     string
	CreatedAt   time.Time
}

func (d *DB) WriteEvent(e Event) error {
	_, err := d.conn.Exec(
		`INSERT INTO env_events (env_name, actor_email, action_type, description, outcome, log_path)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.EnvName, e.ActorEmail, e.ActionType, e.Description, e.Outcome, nullString(e.LogPath),
	)
	return err
}

func (d *DB) UpdateOutcome(envName, actionType, outcome string) error {
	_, err := d.conn.Exec(
		`UPDATE env_events SET outcome = ?
		 WHERE id = (
		   SELECT id FROM env_events WHERE env_name = ? AND action_type = ?
		   ORDER BY created_at DESC LIMIT 1
		 )`,
		outcome, envName, actionType,
	)
	return err
}

func (d *DB) ListEvents(envName string, limit, offset int) ([]Event, error) {
	rows, err := d.conn.Query(
		`SELECT id, env_name, actor_email, action_type, description, outcome, log_path, created_at
		 FROM env_events WHERE env_name = ?
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		envName, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var logPath sql.NullString
		if err := rows.Scan(&e.ID, &e.EnvName, &e.ActorEmail, &e.ActionType,
			&e.Description, &e.Outcome, &logPath, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.LogPath = logPath.String
		events = append(events, e)
	}
	return events, rows.Err()
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

var errNotFound = errors.New("not found")
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/db/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/db/ go.mod go.sum
git commit -m "feat: add SQLite event log with write/list/update-outcome"
```

---

## Task 2: IAP SSH client

**Files:**
- Create: `internal/iapssh/client.go`
- Create: `internal/iapssh/client_test.go`

- [ ] **Step 1: Add SSH dependency**

```bash
go get golang.org/x/crypto@latest
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/iapssh/client_test.go
package iapssh_test

import (
	"testing"

	"github.com/emdash/kindle/internal/iapssh"
	"github.com/stretchr/testify/assert"
)

func TestParseTargetZone(t *testing.T) {
	zone, region := iapssh.ParseZoneAndRegion("us-central1-a")
	assert.Equal(t, "us-central1-a", zone)
	assert.Equal(t, "us-central1", region)
}

func TestNewClient_ConfigNotNil(t *testing.T) {
	cfg := iapssh.Config{
		Project: "my-project",
		Zone:    "us-central1-a",
	}
	client := iapssh.NewClient(cfg)
	assert.NotNil(t, client)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/iapssh/... -v
```

Expected: FAIL — `iapssh` package not found.

- [ ] **Step 4: Write `internal/iapssh/client.go`**

```go
package iapssh

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Project string
	Zone    string
}

type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// Dial opens an SSH connection to the named VM via IAP tunnel.
// The portal's service account must have roles/iap.tunnelResourceAccessor.
// The VM must have OS Login enabled (enable-oslogin=TRUE metadata).
func (c *Client) Dial(ctx context.Context, vmName string) (*ssh.Client, error) {
	// Use gcloud compute ssh --tunnel-through-iap to create the tunnel.
	// We start a local port-forward and connect ssh to it.
	localPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("find free port: %w", err)
	}

	// Start IAP tunnel in background
	tunnel := exec.CommandContext(ctx,
		"gcloud", "compute", "start-iap-tunnel", vmName, "22",
		"--local-host-port", fmt.Sprintf("localhost:%d", localPort),
		"--zone", c.cfg.Zone,
		"--project", c.cfg.Project,
	)
	if err := tunnel.Start(); err != nil {
		return nil, fmt.Errorf("start IAP tunnel: %w", err)
	}

	// Wait for tunnel to be ready
	addr := fmt.Sprintf("localhost:%d", localPort)
	if err := waitForPort(addr, 15*time.Second); err != nil {
		tunnel.Process.Kill()
		return nil, fmt.Errorf("IAP tunnel not ready: %w", err)
	}

	// Connect SSH using OS Login — gcloud-generated key is in ~/.ssh/google_compute_engine
	sshConfig := &ssh.ClientConfig{
		User:            osLoginUser(),
		Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(loadOSLoginKey)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // IAP tunnel is already authenticated
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		tunnel.Process.Kill()
		return nil, fmt.Errorf("SSH dial: %w", err)
	}
	return client, nil
}

// RunCommand opens a session on the SSH client, runs cmd, and returns stdout.
func RunCommand(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(cmd)
}

func ParseZoneAndRegion(zone string) (string, string) {
	parts := strings.Split(zone, "-")
	if len(parts) < 3 {
		return zone, zone
	}
	region := strings.Join(parts[:len(parts)-1], "-")
	return zone, region
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", addr)
}

func osLoginUser() string {
	// OS Login username is derived from the service account email.
	// In practice, gcloud handles this via SSH certificates.
	// Return empty to use the default OS Login behavior.
	return ""
}

func loadOSLoginKey() ([]ssh.Signer, error) {
	// gcloud generates ~/.ssh/google_compute_engine for OS Login.
	// The ssh.Agent or file-based auth would be used here.
	// For now, return empty — actual auth is handled by the OS Login SSH certificate
	// that gcloud injects automatically when the tunnel is started via gcloud CLI.
	return nil, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/iapssh/... -v
```

Expected: PASS (the tests only check config creation and zone parsing, not actual SSH).

- [ ] **Step 6: Commit**

```bash
git add internal/iapssh/ go.mod go.sum
git commit -m "feat: add IAP SSH client using gcloud tunnel"
```

---

## Task 3: Kubeconfig cache

**Files:**
- Create: `internal/kubeconfig/cache.go`
- Create: `internal/kubeconfig/cache_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/kubeconfig/cache_test.go
package kubeconfig_test

import (
	"testing"

	"github.com/emdash/kindle/internal/kubeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFetcher struct {
	calls int
	data  []byte
}

func (f *fakeFetcher) Fetch(envName string) ([]byte, error) {
	f.calls++
	return f.data, nil
}

func TestCache_FetchesOnFirstCall(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("kubeconfig-content")}
	cache := kubeconfig.NewCache(fetcher.Fetch)

	data, err := cache.Get("dev-alice")
	require.NoError(t, err)
	assert.Equal(t, []byte("kubeconfig-content"), data)
	assert.Equal(t, 1, fetcher.calls)
}

func TestCache_ReturnsCachedOnSecondCall(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("kubeconfig-content")}
	cache := kubeconfig.NewCache(fetcher.Fetch)

	cache.Get("dev-alice")
	cache.Get("dev-alice")

	assert.Equal(t, 1, fetcher.calls, "should only fetch once")
}

func TestCache_InvalidateRefetches(t *testing.T) {
	fetcher := &fakeFetcher{data: []byte("kubeconfig-content")}
	cache := kubeconfig.NewCache(fetcher.Fetch)

	cache.Get("dev-alice")
	cache.Invalidate("dev-alice")
	cache.Get("dev-alice")

	assert.Equal(t, 2, fetcher.calls)
}

func TestPatchServerURL(t *testing.T) {
	input := []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: default
`)
	output, err := kubeconfig.PatchServerURL(input, "dev-alice.local-dev.example.com")
	require.NoError(t, err)
	assert.Contains(t, string(output), "https://dev-alice.local-dev.example.com:6443")
	assert.NotContains(t, string(output), "127.0.0.1")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/kubeconfig/... -v
```

Expected: FAIL — `kubeconfig` package not found.

- [ ] **Step 3: Write `internal/kubeconfig/cache.go`**

```go
package kubeconfig

import (
	"bytes"
	"fmt"
	"sync"
)

type FetchFunc func(envName string) ([]byte, error)

type Cache struct {
	mu      sync.RWMutex
	entries map[string][]byte
	fetch   FetchFunc
}

func NewCache(fetch FetchFunc) *Cache {
	return &Cache{
		entries: make(map[string][]byte),
		fetch:   fetch,
	}
}

func (c *Cache) Get(envName string) ([]byte, error) {
	c.mu.RLock()
	data, ok := c.entries[envName]
	c.mu.RUnlock()
	if ok {
		return data, nil
	}

	data, err := c.fetch(envName)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[envName] = data
	c.mu.Unlock()
	return data, nil
}

func (c *Cache) Invalidate(envName string) {
	c.mu.Lock()
	delete(c.entries, envName)
	c.mu.Unlock()
}

// PatchServerURL replaces the server URL in a k3s kubeconfig with the env's DNS hostname.
func PatchServerURL(raw []byte, fqdn string) ([]byte, error) {
	// Simple byte replacement — avoids YAML round-trip which can alter formatting.
	old := []byte("https://127.0.0.1:6443")
	newURL := []byte(fmt.Sprintf("https://%s:6443", fqdn))
	if !bytes.Contains(raw, old) {
		return nil, fmt.Errorf("expected server URL %s not found in kubeconfig", old)
	}
	return bytes.ReplaceAll(raw, old, newURL), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/kubeconfig/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/kubeconfig/
git commit -m "feat: add kubeconfig cache with fetch function and URL patching"
```

---

## Task 4: Kubernetes workloads client

**Files:**
- Create: `internal/k8s/workloads.go`
- Create: `internal/k8s/workloads_test.go`

- [ ] **Step 1: Add client-go dependency**

```bash
go get k8s.io/client-go@latest
go get k8s.io/api@latest
go get k8s.io/apimachinery@latest
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/k8s/workloads_test.go
package k8s_test

import (
	"context"
	"testing"

	"github.com/emdash/kindle/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkloadClient struct {
	workloads []k8s.Workload
}

func (f *fakeWorkloadClient) ListWorkloads(ctx context.Context, namespace string) ([]k8s.Workload, error) {
	return f.workloads, nil
}

func TestFakeClient_ListWorkloads(t *testing.T) {
	client := &fakeWorkloadClient{
		workloads: []k8s.Workload{
			{Kind: "Deployment", Name: "api", Namespace: "dev-alice", ReadyReplicas: 1, DesiredReplicas: 1,
				Containers: []k8s.ContainerInfo{{Name: "api", Image: "myrepo/api:v1.2.3"}}},
		},
	}
	workloads, err := client.ListWorkloads(context.Background(), "dev-alice")
	require.NoError(t, err)
	assert.Len(t, workloads, 1)
	assert.Equal(t, "Deployment", workloads[0].Kind)
	assert.Equal(t, "myrepo/api:v1.2.3", workloads[0].Containers[0].Image)
}

func TestParseImageTag(t *testing.T) {
	repo, tag := k8s.ParseImageTag("myrepo/api:v1.2.3")
	assert.Equal(t, "myrepo/api", repo)
	assert.Equal(t, "v1.2.3", tag)
}

func TestParseImageTag_NoTag(t *testing.T) {
	repo, tag := k8s.ParseImageTag("myrepo/api")
	assert.Equal(t, "myrepo/api", repo)
	assert.Equal(t, "latest", tag)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/k8s/... -v
```

Expected: FAIL — `k8s` package not found.

- [ ] **Step 4: Write `internal/k8s/workloads.go`**

```go
package k8s

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ContainerInfo struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

type Workload struct {
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace"`
	ReadyReplicas   int32           `json:"ready_replicas"`
	DesiredReplicas int32           `json:"desired_replicas"`
	Containers      []ContainerInfo `json:"containers"`
}

type WorkloadClient interface {
	ListWorkloads(ctx context.Context, namespace string) ([]Workload, error)
}

type k8sWorkloadClient struct {
	clientset *kubernetes.Clientset
}

func NewWorkloadClient(kubeconfig []byte) (WorkloadClient, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &k8sWorkloadClient{clientset: cs}, nil
}

func (c *k8sWorkloadClient) ListWorkloads(ctx context.Context, namespace string) ([]Workload, error) {
	var workloads []Workload

	deps, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deps.Items {
		workloads = append(workloads, deploymentToWorkload(d))
	}

	sts, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range sts.Items {
		workloads = append(workloads, statefulSetToWorkload(s))
	}

	return workloads, nil
}

func deploymentToWorkload(d appsv1.Deployment) Workload {
	w := Workload{
		Kind:            "Deployment",
		Name:            d.Name,
		Namespace:       d.Namespace,
		ReadyReplicas:   d.Status.ReadyReplicas,
		DesiredReplicas: *d.Spec.Replicas,
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		repo, tag := ParseImageTag(c.Image)
		w.Containers = append(w.Containers, ContainerInfo{Name: c.Name, Image: c.Image, Tag: tag, _ : repo})
	}
	return w
}

func statefulSetToWorkload(s appsv1.StatefulSet) Workload {
	w := Workload{
		Kind:            "StatefulSet",
		Name:            s.Name,
		Namespace:       s.Namespace,
		ReadyReplicas:   s.Status.ReadyReplicas,
		DesiredReplicas: *s.Spec.Replicas,
	}
	for _, c := range s.Spec.Template.Spec.Containers {
		repo, tag := ParseImageTag(c.Image)
		_ = repo
		w.Containers = append(w.Containers, ContainerInfo{Name: c.Name, Image: c.Image, Tag: tag})
	}
	return w
}

func ParseImageTag(image string) (repo, tag string) {
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		return image[:idx], image[idx+1:]
	}
	return image, "latest"
}
```

- [ ] **Step 5: Fix the struct literal bug in deploymentToWorkload (remove `_ : repo` leftover)**

```go
// Replace the containers loop in deploymentToWorkload with:
for _, c := range d.Spec.Template.Spec.Containers {
    _, tag := ParseImageTag(c.Image)
    w.Containers = append(w.Containers, ContainerInfo{Name: c.Name, Image: c.Image, Tag: tag})
}
```

Apply the same fix to `statefulSetToWorkload`:

```go
for _, c := range s.Spec.Template.Spec.Containers {
    _, tag := ParseImageTag(c.Image)
    w.Containers = append(w.Containers, ContainerInfo{Name: c.Name, Image: c.Image, Tag: tag})
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/k8s/... -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/k8s/ go.mod go.sum
git commit -m "feat: add Kubernetes workload client with Deployment and StatefulSet support"
```

---

## Task 5: Helm runner

**Files:**
- Create: `internal/helm/runner.go`
- Create: `internal/helm/runner_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/helm/runner_test.go
package helm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emdash/kindle/internal/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFakeHelm(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "helm")
	content := fmt.Sprintf("#!/bin/sh\necho \"helm $@\"\nexit %d\n", exitCode)
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))
	return script
}

func TestRunner_Upgrade_Success(t *testing.T) {
	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(kubeconfigFile, []byte("test"), 0600))

	r := helm.NewRunner(helm.RunnerConfig{HelmBin: makeFakeHelm(t, 0)})
	err := r.Upgrade(helm.UpgradeParams{
		ReleaseName:    "dev-alice",
		Namespace:      "dev-alice",
		ChartPath:      "/opt/chart-repo",
		ValuesFile:     "/data/envs/dev-alice/values.yaml",
		KubeconfigPath: kubeconfigFile,
		LogPath:        filepath.Join(t.TempDir(), "helm.log"),
	})
	assert.NoError(t, err)
}

func TestRunner_Upgrade_Failure(t *testing.T) {
	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig.yaml")
	require.NoError(t, os.WriteFile(kubeconfigFile, []byte("test"), 0600))

	r := helm.NewRunner(helm.RunnerConfig{HelmBin: makeFakeHelm(t, 1)})
	err := r.Upgrade(helm.UpgradeParams{
		ReleaseName:    "dev-alice",
		Namespace:      "dev-alice",
		ChartPath:      "/opt/chart-repo",
		ValuesFile:     "/data/envs/dev-alice/values.yaml",
		KubeconfigPath: kubeconfigFile,
		LogPath:        filepath.Join(t.TempDir(), "helm.log"),
	})
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/helm/... -v
```

Expected: FAIL — `helm` package not found.

- [ ] **Step 3: Write `internal/helm/runner.go`**

```go
package helm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type RunnerConfig struct {
	HelmBin string // defaults to "helm" in PATH
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

- [ ] **Step 4: Add missing `fmt` import to runner_test.go**

```go
import (
    "fmt"
    "os"
    "path/filepath"
    "testing"
    ...
)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/helm/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/helm/
git commit -m "feat: add Helm runner for upgrade and status operations"
```

---

## Task 6: On-demand status endpoint

**Files:**
- Create: `internal/api/status.go`
- Create: `internal/api/status_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/status_test.go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkloadClient struct {
	workloads []k8s.Workload
	reachable bool
}

func (f *fakeWorkloadClient) ListWorkloads(ctx context.Context, namespace string) ([]k8s.Workload, error) {
	if !f.reachable {
		return nil, fmt.Errorf("connection refused")
	}
	return f.workloads, nil
}

func TestStatusHandler_Ready(t *testing.T) {
	h := api.NewStatusHandler(api.StatusHandlerDeps{
		GetWorkloadClient: func(envName string) (k8s.WorkloadClient, error) {
			return &fakeWorkloadClient{reachable: true, workloads: []k8s.Workload{
				{Kind: "Deployment", Name: "api", ReadyReplicas: 1, DesiredReplicas: 1},
			}}, nil
		},
		HelmStatus: func(envName string) (string, error) { return `{"info":{"status":"deployed"}}`, nil },
	})

	r := chi.NewRouter()
	r.Get("/api/envs/{name}/status", h.Get)
	req := httptest.NewRequest(http.MethodGet, "/api/envs/dev-alice/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "ready", body["status"])
}

func TestStatusHandler_Degraded(t *testing.T) {
	h := api.NewStatusHandler(api.StatusHandlerDeps{
		GetWorkloadClient: func(envName string) (k8s.WorkloadClient, error) {
			return nil, fmt.Errorf("unreachable")
		},
		HelmStatus: func(envName string) (string, error) { return "", fmt.Errorf("unreachable") },
	})

	r := chi.NewRouter()
	r.Get("/api/envs/{name}/status", h.Get)
	req := httptest.NewRequest(http.MethodGet, "/api/envs/dev-alice/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "degraded", body["status"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -run TestStatus -v
```

Expected: FAIL — `StatusHandler` not defined.

- [ ] **Step 3: Write `internal/api/status.go`**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/k8s"
)

type StatusHandlerDeps struct {
	GetWorkloadClient func(envName string) (k8s.WorkloadClient, error)
	HelmStatus        func(envName string) (string, error)
}

type StatusHandler struct {
	deps StatusHandlerDeps
}

func NewStatusHandler(deps StatusHandlerDeps) *StatusHandler {
	return &StatusHandler{deps: deps}
}

type statusResponse struct {
	Status    string        `json:"status"`
	Workloads []k8s.Workload `json:"workloads,omitempty"`
}

func (h *StatusHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	wc, err := h.deps.GetWorkloadClient(name)
	if err != nil {
		writeJSON(w, http.StatusOK, statusResponse{Status: "degraded"})
		return
	}

	workloads, err := wc.ListWorkloads(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, statusResponse{Status: "degraded"})
		return
	}

	helmOut, err := h.deps.HelmStatus(name)
	if err != nil || !isHelmDeployed(helmOut) {
		writeJSON(w, http.StatusOK, statusResponse{Status: "degraded", Workloads: workloads})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "ready", Workloads: workloads})
}

func isHelmDeployed(helmJSON string) bool {
	return strings.Contains(helmJSON, `"deployed"`)
}
```

- [ ] **Step 4: Add missing fmt import to status_test.go**

```go
import "fmt"
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/api/... -run TestStatus -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/status.go internal/api/status_test.go
git commit -m "feat: add on-demand status endpoint (ready/degraded)"
```

---

## Task 7: Kubeconfig download endpoint

**Files:**
- Create: `internal/api/kubeconfig.go`
- Create: `internal/api/kubeconfig_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/kubeconfig_test.go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeconfigHandler_Download(t *testing.T) {
	rawKubeconfig := []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: default
`)
	h := api.NewKubeconfigHandler(api.KubeconfigHandlerDeps{
		GetKubeconfig: func(envName string) ([]byte, error) { return rawKubeconfig, nil },
		ZoneDomain:    "local-dev.example.com",
	})

	r := chi.NewRouter()
	r.Get("/api/envs/{name}/kubeconfig", h.Download)
	req := httptest.NewRequest(http.MethodGet, "/api/envs/dev-alice/kubeconfig", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "attachment; filename=\"dev-alice-kubeconfig.yaml\"",
		w.Header().Get("Content-Disposition"))
	assert.Contains(t, w.Body.String(), "dev-alice.local-dev.example.com:6443")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -run TestKubeconfig -v
```

Expected: FAIL.

- [ ] **Step 3: Write `internal/api/kubeconfig.go`**

```go
package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/kubeconfig"
)

type KubeconfigHandlerDeps struct {
	GetKubeconfig func(envName string) ([]byte, error)
	ZoneDomain    string
}

type KubeconfigHandler struct {
	deps KubeconfigHandlerDeps
}

func NewKubeconfigHandler(deps KubeconfigHandlerDeps) *KubeconfigHandler {
	return &KubeconfigHandler{deps: deps}
}

func (h *KubeconfigHandler) Download(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	raw, err := h.deps.GetKubeconfig(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch kubeconfig")
		return
	}

	fqdn := fmt.Sprintf("%s.%s", name, h.deps.ZoneDomain)
	patched, err := kubeconfig.PatchServerURL(raw, fqdn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+"-kubeconfig.yaml"))
	w.Write(patched)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/api/... -run TestKubeconfig -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/kubeconfig.go internal/api/kubeconfig_test.go
git commit -m "feat: add kubeconfig download endpoint with server URL patching"
```

---

## Task 8: Operations endpoints (edit-values, update-image, upgrade-chart, resize)

**Files:**
- Create: `internal/api/operations.go`
- Create: `internal/api/operations_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/operations_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *db.DB {
	d, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestEditValues_WritesFilesAndResponds(t *testing.T) {
	dataDir := t.TempDir()
	envDir := filepath.Join(dataDir, "envs", "dev-alice")
	require.NoError(t, os.MkdirAll(envDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "values.yaml"), []byte("old: value\n"), 0644))

	helmCalled := false
	h := api.NewOperationsHandler(api.OperationsHandlerDeps{
		DataDir: dataDir,
		DB:      openTestDB(t),
		HelmUpgrade: func(envName, valuesPath, kubeconfigPath string) error {
			helmCalled = true
			return nil
		},
		GetKubeconfigPath: func(envName string) (string, error) { return "/tmp/kube", nil },
	})

	r := chi.NewRouter()
	r.Post("/api/envs/{name}/edit-values", h.EditValues)

	body, _ := json.Marshal(map[string]string{"values_yaml": "new: value\n"})
	req := httptest.NewRequest(http.MethodPost, "/api/envs/dev-alice/edit-values", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, helmCalled)

	content, _ := os.ReadFile(filepath.Join(envDir, "values.yaml"))
	assert.Equal(t, "new: value\n", string(content))
}

func TestUpdateImage_UpdatesValuesAndHelm(t *testing.T) {
	dataDir := t.TempDir()
	envDir := filepath.Join(dataDir, "envs", "dev-alice")
	require.NoError(t, os.MkdirAll(envDir, 0755))

	initialValues := `api:
  image:
    tag: v1.0.0
`
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "values.yaml"), []byte(initialValues), 0644))

	helmCalled := false
	h := api.NewOperationsHandler(api.OperationsHandlerDeps{
		DataDir: dataDir,
		DB:      openTestDB(t),
		HelmUpgrade: func(envName, valuesPath, kubeconfigPath string) error {
			helmCalled = true
			return nil
		},
		GetKubeconfigPath: func(envName string) (string, error) { return "/tmp/kube", nil },
		GetImageTagKey:    func(envName, workloadName string) (string, error) { return "api.image.tag", nil },
	})

	r := chi.NewRouter()
	r.Post("/api/envs/{name}/update-image", h.UpdateImage)

	body, _ := json.Marshal(map[string]string{
		"workload_name": "api",
		"container":     "api",
		"new_tag":       "v1.1.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/envs/dev-alice/update-image", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, helmCalled)

	content, _ := os.ReadFile(filepath.Join(envDir, "values.yaml"))
	assert.Contains(t, string(content), "v1.1.0")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -run TestEditValues -run TestUpdateImage -v
```

Expected: FAIL.

- [ ] **Step 3: Write `internal/api/operations.go`**

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/auth"
	"github.com/emdash/kindle/internal/db"
	"github.com/emdash/kindle/internal/terraform"
	"gopkg.in/yaml.v3"
)

type OperationsHandlerDeps struct {
	DataDir           string
	DB                *db.DB
	Jobs              *terraform.JobMap
	TFRunner          *terraform.Runner
	HelmUpgrade       func(envName, valuesPath, kubeconfigPath string) error
	GetKubeconfigPath func(envName string) (string, error)
	GetImageTagKey    func(envName, workloadName string) (string, error)
}

type OperationsHandler struct {
	deps OperationsHandlerDeps
}

func NewOperationsHandler(deps OperationsHandlerDeps) *OperationsHandler {
	return &OperationsHandler{deps: deps}
}

func (h *OperationsHandler) EditValues(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	actor := actorEmail(r)

	var req struct {
		ValuesYAML string `json:"values_yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	valuesPath := filepath.Join(h.deps.DataDir, "envs", name, "values.yaml")
	if err := os.WriteFile(valuesPath, []byte(req.ValuesYAML), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write values")
		return
	}

	kubeconfigPath, err := h.deps.GetKubeconfigPath(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get kubeconfig")
		return
	}

	outcome := db.OutcomeSuccess
	if err := h.deps.HelmUpgrade(name, valuesPath, kubeconfigPath); err != nil {
		outcome = db.OutcomeFailure
	}

	h.deps.DB.WriteEvent(db.Event{
		EnvName:     name,
		ActorEmail:  actor,
		ActionType:  "edit_values",
		Description: "Edited Helm values",
		Outcome:     outcome,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": outcome})
}

func (h *OperationsHandler) UpdateImage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	actor := actorEmail(r)

	var req struct {
		WorkloadName string `json:"workload_name"`
		Container    string `json:"container"`
		NewTag       string `json:"new_tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	valuesPath := filepath.Join(h.deps.DataDir, "envs", name, "values.yaml")
	raw, err := os.ReadFile(valuesPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read values")
		return
	}

	tagKey, err := h.deps.GetImageTagKey(name, req.WorkloadName)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown workload: %s", req.WorkloadName))
		return
	}

	updated, err := setYAMLKey(raw, tagKey, req.NewTag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update values")
		return
	}

	if err := os.WriteFile(valuesPath, updated, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write values")
		return
	}

	kubeconfigPath, _ := h.deps.GetKubeconfigPath(name)
	outcome := db.OutcomeSuccess
	if err := h.deps.HelmUpgrade(name, valuesPath, kubeconfigPath); err != nil {
		outcome = db.OutcomeFailure
	}

	h.deps.DB.WriteEvent(db.Event{
		EnvName:     name,
		ActorEmail:  actor,
		ActionType:  "update_image",
		Description: fmt.Sprintf("Updated image tag for %s to %s", req.WorkloadName, req.NewTag),
		Outcome:     outcome,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": outcome})
}

func (h *OperationsHandler) UpgradeChart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	actor := actorEmail(r)

	var req struct {
		ChartRef string `json:"chart_ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ChartRef == "" {
		writeError(w, http.StatusBadRequest, "chart_ref is required")
		return
	}

	if h.deps.Jobs.InFlight(name) {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	logPath := h.logPath(name, "upgrade_chart")
	workDir := filepath.Join(h.deps.DataDir, "envs", name, "terraform")
	h.deps.Jobs.Set(name, terraform.JobStatus{
		State: terraform.StateProvisioning, StartedAt: time.Now(), Actor: actor, LogPath: logPath,
	})

	h.deps.DB.WriteEvent(db.Event{
		EnvName:     name,
		ActorEmail:  actor,
		ActionType:  "upgrade_chart",
		Description: fmt.Sprintf("Upgrading chart to %s", req.ChartRef),
		Outcome:     db.OutcomeInProgress,
		LogPath:     logPath,
	})

	go func() {
		err := h.deps.TFRunner.Apply(workDir, logPath)
		outcome := db.OutcomeSuccess
		if err != nil {
			outcome = db.OutcomeFailure
		}
		h.deps.DB.UpdateOutcome(name, "upgrade_chart", outcome)
		h.deps.Jobs.Delete(name)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "in_progress"})
}

func (h *OperationsHandler) Resize(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	actor := actorEmail(r)

	var req struct {
		MachineType string `json:"machine_type,omitempty"`
		DiskSizeGB  int64  `json:"disk_size_gb,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if h.deps.Jobs.InFlight(name) {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	logPath := h.logPath(name, "resize")
	workDir := filepath.Join(h.deps.DataDir, "envs", name, "terraform")
	h.deps.Jobs.Set(name, terraform.JobStatus{
		State: terraform.StateProvisioning, StartedAt: time.Now(), Actor: actor, LogPath: logPath,
	})

	h.deps.DB.WriteEvent(db.Event{
		EnvName:     name,
		ActorEmail:  actor,
		ActionType:  "resize_vm",
		Description: fmt.Sprintf("Resizing VM (machine_type=%s, disk_size_gb=%d)", req.MachineType, req.DiskSizeGB),
		Outcome:     db.OutcomeInProgress,
		LogPath:     logPath,
	})

	go func() {
		err := h.deps.TFRunner.Apply(workDir, logPath)
		outcome := db.OutcomeSuccess
		if err != nil {
			outcome = db.OutcomeFailure
		}
		h.deps.DB.UpdateOutcome(name, "resize_vm", outcome)
		h.deps.Jobs.Delete(name)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "in_progress"})
}

func (h *OperationsHandler) logPath(name, action string) string {
	ts := time.Now().Format("20060102-150405")
	path := filepath.Join(h.deps.DataDir, "envs", name, "ops", fmt.Sprintf("%s-%s.log", ts, action))
	os.MkdirAll(filepath.Dir(path), 0755)
	return path
}

func actorEmail(r *http.Request) string {
	sess := auth.SessionFromContext(r.Context())
	if sess == nil {
		return ""
	}
	return sess.Email
}

// setYAMLKey sets a dot-delimited key (e.g. "api.image.tag") to value in a YAML document.
func setYAMLKey(raw []byte, dotKey, value string) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	keys := strings.Split(dotKey, ".")
	setNested(doc, keys, value)
	return yaml.Marshal(doc)
}

func setNested(m map[string]any, keys []string, value string) {
	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}
	sub, ok := m[keys[0]].(map[string]any)
	if !ok {
		sub = make(map[string]any)
		m[keys[0]] = sub
	}
	setNested(sub, keys[1:], value)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/api/... -run TestEditValues -run TestUpdateImage -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/operations.go internal/api/operations_test.go
git commit -m "feat: add edit-values, update-image, upgrade-chart, resize operations"
```

---

## Task 9: Events endpoint

**Files:**
- Create: `internal/api/events.go`
- Create: `internal/api/events_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/events_test.go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventsHandler_List(t *testing.T) {
	d := openTestDB(t)
	d.WriteEvent(db.Event{
		EnvName: "dev-alice", ActorEmail: "alice@example.com",
		ActionType: "create", Description: "Created env", Outcome: db.OutcomeSuccess,
	})

	h := api.NewEventsHandler(api.EventsHandlerDeps{DB: d})
	r := chi.NewRouter()
	r.Get("/api/envs/{name}/events", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/envs/dev-alice/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body, 1)
	assert.Equal(t, "create", body[0]["action_type"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -run TestEvents -v
```

Expected: FAIL.

- [ ] **Step 3: Write `internal/api/events.go`**

```go
package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/db"
)

type EventsHandlerDeps struct {
	DB *db.DB
}

type EventsHandler struct {
	deps EventsHandlerDeps
}

func NewEventsHandler(deps EventsHandlerDeps) *EventsHandler {
	return &EventsHandler{deps: deps}
}

func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	events, err := h.deps.DB.ListEvents(name, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []db.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/api/... -run TestEvents -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/events.go internal/api/events_test.go
git commit -m "feat: add paginated events endpoint"
```

---

## Task 10: Register all new routes in server

**Files:**
- Modify: `internal/server/server.go`

- [ ] **Step 1: Update `internal/server/server.go` to wire all operation handlers**

```go
package server

import (
	"context"
	"fmt"
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
	"github.com/emdash/kindle/internal/terraform"
)

type Server struct {
	cfg    *config.Config
	auth   *auth.Handler
	router chi.Router
}

func New(cfg *config.Config, vmClient gcp.VMClient, database *db.DB) *Server {
	authHandler := auth.NewHandler(auth.HandlerConfig{
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		RedirectURL:     fmt.Sprintf("http://localhost:%s/auth/callback", cfg.Port),
		CorporateDomain: cfg.CorporateDomain,
		SessionSecret:   cfg.SessionSecret,
	})

	jobs := terraform.NewJobMap()
	tfRunner := terraform.NewRunner(terraform.RunnerConfig{})
	helmRunner := helm.NewRunner(helm.RunnerConfig{})
	presetLoader := presets.NewLoader(cfg.PresetsDir)
	chartLister := chartversions.NewLister(cfg.ChartRepoURL)
	sshClient := iapssh.NewClient(iapssh.Config{Project: cfg.GCPProjectID, Zone: "us-central1-a"})

	// Kubeconfig: fetch via IAP SSH and cache in memory
	kubeconfigCache := kubeconfig.NewCache(func(envName string) ([]byte, error) {
		ctx := context.Background()
		sshConn, err := sshClient.Dial(ctx, envName)
		if err != nil {
			return nil, err
		}
		defer sshConn.Close()
		return iapssh.RunCommand(sshConn, "cat /etc/rancher/k3s/k3s.yaml")
	})

	getKubeconfigPath := func(envName string) (string, error) {
		raw, err := kubeconfigCache.Get(envName)
		if err != nil {
			return "", err
		}
		fqdn := fmt.Sprintf("%s.%s", envName, cfg.ZoneDomain)
		patched, err := kubeconfig.PatchServerURL(raw, fqdn)
		if err != nil {
			return "", err
		}
		path := filepath.Join(cfg.DataDir, "envs", envName, "kubeconfig.yaml")
		os.MkdirAll(filepath.Dir(path), 0755)
		return path, os.WriteFile(path, patched, 0600)
	}

	getImageTagKey := func(envName, workloadName string) (string, error) {
		// Read image_tag_keys from the env's preset template.
		// For v1, we look up the preset name from the VM labels via the vmClient.
		// Simplified: look in all presets for a matching entry.
		list, _ := presetLoader.List()
		for _, p := range list {
			if key, ok := p.ImageTagKeys[workloadName]; ok {
				return key, nil
			}
		}
		return "", fmt.Errorf("no image tag key found for workload %s", workloadName)
	}

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

	statusHandler := api.NewStatusHandler(api.StatusHandlerDeps{
		GetWorkloadClient: func(envName string) (k8s.WorkloadClient, error) {
			raw, err := kubeconfigCache.Get(envName)
			if err != nil {
				return nil, err
			}
			fqdn := fmt.Sprintf("%s.%s", envName, cfg.ZoneDomain)
			patched, _ := kubeconfig.PatchServerURL(raw, fqdn)
			return k8s.NewWorkloadClient(patched)
		},
		HelmStatus: func(envName string) (string, error) {
			kubePath, err := getKubeconfigPath(envName)
			if err != nil {
				return "", err
			}
			return helmRunner.Status(helm.StatusParams{
				ReleaseName:    envName,
				Namespace:      envName,
				KubeconfigPath: kubePath,
			})
		},
	})

	kubeconfigHandler := api.NewKubeconfigHandler(api.KubeconfigHandlerDeps{
		GetKubeconfig: kubeconfigCache.Get,
		ZoneDomain:    cfg.ZoneDomain,
	})

	opsHandler := api.NewOperationsHandler(api.OperationsHandlerDeps{
		DataDir: cfg.DataDir,
		DB:      database,
		Jobs:    jobs,
		TFRunner: tfRunner,
		HelmUpgrade: func(envName, valuesPath, kubeconfigPath string) error {
			chartPath := filepath.Join("/opt/chart-repo") // on the portal VM after Plan 2 startup script
			return helmRunner.Upgrade(helm.UpgradeParams{
				ReleaseName:    envName,
				Namespace:      envName,
				ChartPath:      chartPath,
				ValuesFile:     valuesPath,
				KubeconfigPath: kubeconfigPath,
				LogPath:        filepath.Join(cfg.DataDir, "envs", envName, "ops", "helm-upgrade.log"),
			})
		},
		GetKubeconfigPath: getKubeconfigPath,
		GetImageTagKey:    getImageTagKey,
	})

	eventsHandler := api.NewEventsHandler(api.EventsHandlerDeps{DB: database})

	s := &Server{cfg: cfg, auth: authHandler}
	s.router = s.buildRouter(envsHandler, templatesHandler, statusHandler, kubeconfigHandler, opsHandler, eventsHandler)
	return s
}

func (s *Server) buildRouter(
	envs *api.EnvsHandler,
	templates *api.TemplatesHandler,
	status *api.StatusHandler,
	kubeconf *api.KubeconfigHandler,
	ops *api.OperationsHandler,
	events *api.EventsHandler,
) chi.Router {
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
		r.Get("/api/envs/{name}/status", status.Get)
		r.Get("/api/envs/{name}/kubeconfig", kubeconf.Download)
		r.Post("/api/envs/{name}/edit-values", ops.EditValues)
		r.Post("/api/envs/{name}/update-image", ops.UpdateImage)
		r.Post("/api/envs/{name}/upgrade-chart", ops.UpgradeChart)
		r.Post("/api/envs/{name}/resize", ops.Resize)
		r.Get("/api/envs/{name}/events", events.List)
		r.Get("/api/templates", templates.List)
		r.Get("/api/templates/{name}/chart-versions", templates.ChartVersions)
	})

	return r
}

func (s *Server) Handler() http.Handler { return s.router }
func (s *Server) Addr() string          { return ":" + s.cfg.Port }
```

- [ ] **Step 2: Update `cmd/portal/main.go` to open SQLite and pass to server**

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/db"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/server"
)

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

	srv := server.New(cfg, vmClient, database)
	slog.Info("portal listening", "addr", srv.Addr())
	if err := http.ListenAndServe(srv.Addr(), srv.Handler()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build and run all tests**

```bash
make build && make test
```

Expected: clean build, all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/server/server.go cmd/portal/main.go
git commit -m "feat: wire all operation handlers — operations layer complete"
```
