package main

// @title Maelstrom API
// @version 1.0.0
// @description Minimal Go HTTP server.
// @host localhost:8080
// @BasePath /

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/comalice/maelstrom/config"
	"github.com/kelseyhightower/envconfig"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		slog.Error("failed to process config", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	slog.Info("Starting server", "addr", cfg.ListenAddr)

	// @Summary Root endpoint
	// @Description Returns hello message
	// @Produce text/plain
	// @Success 200 {string} string "Hello, Maelstrom!"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, Maelstrom!")
	})

	// @Summary Greet user
	// @Description Greet user by name
	// @Tags api
	// @Accept json
	// @Produce json
	// @Param name body string true "User name"
	// @Success 200 {object} map[string]string "greeting"
	// @Failure 400 {string} string "Invalid JSON"
	// @Failure 405 {string} string "Method not allowed"
	http.HandleFunc("/api/v1/greet", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type Request struct {
			Name string `json:"name"`
		}
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		type Response struct {
			Greeting string `json:"greeting"`
		}
		resp := Response{Greeting: "Hello, " + req.Name + "!"}
		if err := json.NewEncoder(w).Encode(&resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Swagger UI
	http.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../docs/swagger.json")
	})
	http.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
	))

	if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
}
