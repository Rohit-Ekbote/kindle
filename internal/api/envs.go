package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/terraform"
	"github.com/go-chi/chi/v5"
)

type EnvsHandlerDeps struct {
	VMClient   gcp.VMClient
	Jobs       *terraform.JobMap
	TFRunner   *terraform.Runner
	ModuleDir  string
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
	Name      string         `json:"name"`
	Template  string         `json:"template"`
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

	_, err := h.deps.VMClient.GetVM(r.Context(), req.Name)
	if err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("env %s already exists", req.Name))
		return
	}
	if !errors.Is(err, gcp.ErrVMNotFound) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

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
