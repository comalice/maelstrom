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
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/comalice/maelstrom/registry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	v1 "github.com/comalice/maelstrom/api/v1"
	"github.com/comalice/maelstrom/config"
	swagger "github.com/comalice/maelstrom/swagger"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "config" {
		fields := config.AppConfigFields()
		if len(os.Args) > 2 && os.Args[2] == "--json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(fields); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
		fmt.Fprintln(w, "Field\t|\tType\t|\tEnv\t|\tDesc\t|\tDefault")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for _, f := range fields {
			fmt.Fprintf(w, "%s\t|\t%s\t|\t%s\t|\t%s\t|\t%s\n",
				f.Field, f.Type, f.Env, f.Desc, f.Default)
		}
		w.Flush()
		os.Exit(0)
	}

	var cfg config.AppConfig
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
	r.Mount("/config-docs", v1.ConfigRouter())

	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
}
