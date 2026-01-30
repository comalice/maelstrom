package main

// @title Maelstrom API
// @version 1.0.0
// @description Minimal Go HTTP server.
// @host localhost:8080
// @BasePath /

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/comalice/maelstrom/registry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	v1 "github.com/comalice/maelstrom/api/v1"
	"github.com/comalice/maelstrom/config"
	"github.com/kelseyhightower/envconfig"
	swagger "github.com/comalice/maelstrom/swagger"
)

func main() {
	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		slog.Error("failed to process config", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	slog.Info("Starting server", "addr", cfg.ListenAddr)

	if cfg.RegistryDir == "" {
		cfg.RegistryDir = "./yaml"
	}
	if err := os.MkdirAll(cfg.RegistryDir, 0755); err != nil {
		slog.Error("failed to create registry dir", "error", err)
		os.Exit(1)
	}
	reg := registry.New()
	if err := reg.InitWatcher(cfg.RegistryDir); err != nil {
		slog.Error("failed to init registry watcher", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()

	// Optional: Basic middleware for logging and panic recovery
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", v1.HelloHandler)

	r.Mount("/api/v1", v1.Router())

	r.Mount("/swagger", swagger.Router())

	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
}
