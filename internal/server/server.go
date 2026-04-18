package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/auth"
	"github.com/emdash/kindle/internal/chartversions"
	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/presets"
	"github.com/emdash/kindle/internal/terraform"
)

type Server struct {
	cfg    *config.Config
	auth   *auth.Handler
	router chi.Router
}

func New(cfg *config.Config, vmClient gcp.VMClient) *Server {
	authHandler := auth.NewHandler(auth.HandlerConfig{
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		RedirectURL:     fmt.Sprintf("http://localhost:%s/auth/callback", cfg.Port),
		CorporateDomain: cfg.CorporateDomain,
		SessionSecret:   cfg.SessionSecret,
	})

	jobs := terraform.NewJobMap()
	tfRunner := terraform.NewRunner(terraform.RunnerConfig{})

	presetLoader := presets.NewLoader(cfg.PresetsDir)

	var chartLister *chartversions.Lister
	if lister, err := chartversions.NewLister(cfg.ChartRepoURL); err != nil {
		slog.Warn("chart versions lister unavailable", "error", err)
	} else {
		chartLister = lister
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

	s := &Server{cfg: cfg, auth: authHandler}
	s.router = s.buildRouter(envsHandler, templatesHandler)
	return s
}

func (s *Server) buildRouter(envs *api.EnvsHandler, templates *api.TemplatesHandler) chi.Router {
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
		r.Get("/api/templates", templates.List)
		r.Get("/api/templates/{name}/chart-versions", templates.ChartVersions)
	})

	return r
}

func (s *Server) Handler() http.Handler { return s.router }
func (s *Server) Addr() string          { return ":" + s.cfg.Port }
