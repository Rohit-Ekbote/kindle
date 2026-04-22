package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emdash/kindle/internal/auth"
	"github.com/emdash/kindle/internal/db"
	"github.com/emdash/kindle/internal/terraform"
	"github.com/go-chi/chi/v5"
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

	if h.deps.Jobs != nil && h.deps.Jobs.InFlight(name) {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	logPath := h.logPath(name, "upgrade_chart")
	workDir := filepath.Join(h.deps.DataDir, "envs", name, "terraform")
	if h.deps.Jobs != nil {
		h.deps.Jobs.Set(name, terraform.JobStatus{
			State: terraform.StateProvisioning, StartedAt: time.Now(), Actor: actor, LogPath: logPath,
		})
	}

	h.deps.DB.WriteEvent(db.Event{
		EnvName:     name,
		ActorEmail:  actor,
		ActionType:  "upgrade_chart",
		Description: fmt.Sprintf("Upgrading chart to %s", req.ChartRef),
		Outcome:     db.OutcomeInProgress,
		LogPath:     logPath,
	})

	jobs := h.deps.Jobs
	tfRunner := h.deps.TFRunner
	database := h.deps.DB
	go func() {
		var err error
		if tfRunner != nil {
			err = tfRunner.Apply(workDir, logPath)
		}
		outcome := db.OutcomeSuccess
		if err != nil {
			outcome = db.OutcomeFailure
		}
		database.UpdateOutcome(name, "upgrade_chart", outcome)
		if jobs != nil {
			jobs.Delete(name)
		}
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

	if h.deps.Jobs != nil && h.deps.Jobs.InFlight(name) {
		writeError(w, http.StatusConflict, "operation already in progress")
		return
	}

	logPath := h.logPath(name, "resize")
	workDir := filepath.Join(h.deps.DataDir, "envs", name, "terraform")
	if h.deps.Jobs != nil {
		h.deps.Jobs.Set(name, terraform.JobStatus{
			State: terraform.StateProvisioning, StartedAt: time.Now(), Actor: actor, LogPath: logPath,
		})
	}

	h.deps.DB.WriteEvent(db.Event{
		EnvName:     name,
		ActorEmail:  actor,
		ActionType:  "resize_vm",
		Description: fmt.Sprintf("Resizing VM (machine_type=%s, disk_size_gb=%d)", req.MachineType, req.DiskSizeGB),
		Outcome:     db.OutcomeInProgress,
		LogPath:     logPath,
	})

	jobs := h.deps.Jobs
	tfRunner := h.deps.TFRunner
	database := h.deps.DB
	go func() {
		var err error
		if tfRunner != nil {
			err = tfRunner.Apply(workDir, logPath)
		}
		outcome := db.OutcomeSuccess
		if err != nil {
			outcome = db.OutcomeFailure
		}
		database.UpdateOutcome(name, "resize_vm", outcome)
		if jobs != nil {
			jobs.Delete(name)
		}
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

func setYAMLKey(raw []byte, dotKey, value string) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = make(map[string]any)
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
