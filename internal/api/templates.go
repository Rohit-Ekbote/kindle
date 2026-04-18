package api

import (
	"net/http"

	"github.com/emdash/kindle/internal/chartversions"
	"github.com/emdash/kindle/internal/presets"
	"github.com/go-chi/chi/v5"
)

type TemplatesHandlerDeps struct {
	Loader *presets.Loader
	Lister *chartversions.Lister
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
		Name         string                      `json:"name"`
		Description  string                      `json:"description"`
		Defaults     any                         `json:"defaults"`
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
	versions, err := h.deps.Lister.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}
