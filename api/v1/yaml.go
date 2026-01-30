package v1

import (
	"encoding/json"
	"net/http"

	"github.com/comalice/maelstrom/registry"
	"github.com/go-chi/chi/v5"
)

// @Summary List YAMLs
// @Description List registry
// @Produce json
// @Success 200 {array} registry.YAMLImport
// @Router /api/v1/yamls [GET]
func ListYamlsHandler(w http.ResponseWriter, r *http.Request) {
	list := registry.GlobalRegistry.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// @Summary Import YAML
// @Description Manual import
// @Param filename path string true "Filename"
// @Produce json
// @Success 200 {string} string
// @Router /api/v1/import/{filename} [POST]
func ImportYamlHandler(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if err := registry.GlobalRegistry.Import(filename); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "imported"})
}
