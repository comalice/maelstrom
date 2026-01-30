// Package v1 provides API v1 routes.
package v1

import (
	"github.com/go-chi/chi/v5"
)

func Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/greet", GreeterHandler)
	return r
}
