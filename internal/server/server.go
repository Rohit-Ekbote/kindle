package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
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

func New(cfg *config.Config, vmClient gcp.VMClient, database *db.DB, staticFiles embed.FS) *Server {
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
	sshClient := iapssh.NewClient(iapssh.Config{Project: cfg.GCPProjectID, Zone: "us-central1-a"})

	var chartLister *chartversions.Lister
	if lister, err := chartversions.NewLister(cfg.ChartRepoURL); err != nil {
		slog.Warn("chart versions lister unavailable", "error", err)
	} else {
		chartLister = lister
	}

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
		DataDir:  cfg.DataDir,
		DB:       database,
		Jobs:     jobs,
		TFRunner: tfRunner,
		HelmUpgrade: func(envName, valuesPath, kubeconfigPath string) error {
			return helmRunner.Upgrade(helm.UpgradeParams{
				ReleaseName:    envName,
				Namespace:      envName,
				ChartPath:      "/opt/chart-repo",
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
	s.router = s.buildRouter(envsHandler, templatesHandler, statusHandler, kubeconfigHandler, opsHandler, eventsHandler, staticFiles)
	return s
}

func (s *Server) buildRouter(
	envs *api.EnvsHandler,
	templates *api.TemplatesHandler,
	status *api.StatusHandler,
	kubeconf *api.KubeconfigHandler,
	ops *api.OperationsHandler,
	events *api.EventsHandler,
	staticFiles embed.FS,
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

	// Serve embedded React SPA for all non-API routes
	webDist, err := fs.Sub(staticFiles, "web/dist")
	if err != nil {
		slog.Error("failed to sub static files", "error", err)
	} else {
		fileServer := http.FileServer(http.FS(webDist))
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			// For client-side routing: serve index.html for unknown paths
			if _, statErr := fs.Stat(webDist, req.URL.Path[1:]); statErr != nil {
				req.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, req)
		})
	}

	return r
}

func (s *Server) Handler() http.Handler { return s.router }
func (s *Server) Addr() string          { return ":" + s.cfg.Port }
