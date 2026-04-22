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
