package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeK8sClient struct {
	workloads []k8s.Workload
	reachable bool
}

func (f *fakeK8sClient) ListWorkloads(ctx context.Context, namespace string) ([]k8s.Workload, error) {
	if !f.reachable {
		return nil, fmt.Errorf("connection refused")
	}
	return f.workloads, nil
}

func TestStatusHandler_Ready(t *testing.T) {
	h := api.NewStatusHandler(api.StatusHandlerDeps{
		GetWorkloadClient: func(envName string) (k8s.WorkloadClient, error) {
			return &fakeK8sClient{reachable: true, workloads: []k8s.Workload{
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
