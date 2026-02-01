package v1

import (
	"net/http"

	"github.com/comalice/maelstrom/config"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// ConfigRouter returns Chi router for /config-docs/ with dynamic OpenAPI schema.
func ConfigRouter() chi.Router {
	r := chi.NewRouter()

	r.Get("/config.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(config.AppConfigSchema())
	})

	r.Mount("/", httpSwagger.Handler(
		httpSwagger.URL("/config-docs/config.json"),
		httpSwagger.DeepLinking(true),
	))

	return r
}
