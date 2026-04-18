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
