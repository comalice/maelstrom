// Package hello provides hello and greeter handlers.
package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// @Summary Root endpoint
// @Description Returns hello message
// @Produce text/plain
// @Success 200 {string} string "Hello, Maelstrom!"
// @Router / [GET]
func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, Maelstrom!")
}

// Request for greeter
type Request struct {
	Name string `json:"name"`
}

// Response for greeter
type Response struct {
	Greeting string `json:"greeting"`
}

// @Summary Greet user
// @Description Greet user by name
// @Tags api
// @Accept json
// @Produce json
// @Param name body Request true "User name"
// @Success 200 {object} Response "greeting"
// @Failure 400 {string} string "Invalid JSON"
// @Failure 405 {string} string "Method not allowed"
// @Router /api/v1/greet [POST]
func GreeterHandler(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	resp := Response{Greeting: "Hello, " + req.Name + "!"}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
