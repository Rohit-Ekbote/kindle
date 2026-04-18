package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/gcp"
	"github.com/emdash/kindle/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	vmClient, err := gcp.NewVMClient(ctx, cfg.GCPProjectID)
	if err != nil {
		slog.Error("failed to create GCP client", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg, vmClient)
	slog.Info("portal listening", "addr", srv.Addr())
	if err := http.ListenAndServe(srv.Addr(), srv.Handler()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
