package api

import (
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
	Status    string         `json:"status"`
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
