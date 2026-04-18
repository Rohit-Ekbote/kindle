package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/db"
	"github.com/go-chi/chi/v5"
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
