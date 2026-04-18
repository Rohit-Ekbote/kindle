package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/db"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/server"
)

//go:embed web/dist
var staticFiles embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	os.MkdirAll(cfg.DataDir, 0755)
	database, err := db.Open(filepath.Join(cfg.DataDir, "portal.db"))
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	ctx := context.Background()
	vmClient, err := gcp.NewVMClient(ctx, cfg.GCPProjectID)
	if err != nil {
		slog.Error("failed to create GCP client", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, vmClient, database, staticFiles)
	slog.Info("portal listening", "addr", srv.Addr())
	if err := http.ListenAndServe(srv.Addr(), srv.Handler()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
