package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/terraform"
	"github.com/go-chi/chi/v5"
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
