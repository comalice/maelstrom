package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/comalice/maelstrom/registry"
	registrystatechart "github.com/comalice/maelstrom/registry/statechart"
	"github.com/comalice/statechartx"
	"github.com/go-chi/chi/v5"
)

var (
	instances         sync.Map // machineID -> map[instID]*Runtime
	nextInstanceID    int64
)

func StatechartsRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", listMachines)
	r.Post("/{machineID}/instances", createInstance)
	r.Post("/{machineID}/instances/{instID}/events", sendEvent)
	return r
}

func getMachines() []string {
	items := registry.GlobalRegistry.List()
	out := []string{}
	for _, item := range items {
		if item.Type == "statechart" && item.Active && item.StatechartAugmented != nil && strings.HasSuffix(item.Filename, ".yaml") {
			out = append(out, strings.TrimSuffix(item.Filename, ".yaml"))
		}
	}
	return out
}

func getAugmentedMachine(id string) (*registrystatechart.AugmentedMachine, error) {
	filename := id + ".yaml"
	items := registry.GlobalRegistry.List()
	for _, item := range items {
		if item.Filename == filename && item.Type == "statechart" && item.Active && item.StatechartAugmented != nil {
			return item.StatechartAugmented, nil
		}
	}
	return nil, fmt.Errorf("machine %q not found", id)
}


func listMachines(w http.ResponseWriter, r *http.Request) {
	mids := getMachines()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mids)
}

type CreateInstanceReq struct {
	InitialContext any `json:"initialContext"`
}

type CreateInstanceResp struct {
	ID     string `json:"id"`
	Current string `json:"current"`
}

func createInstance(w http.ResponseWriter, r *http.Request) {
	mid := chi.URLParam(r, "machineID")
	var req CreateInstanceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	aug, err := getAugmentedMachine(mid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	rt := statechartx.NewRuntime(aug.Machine, req.InitialContext)
	bgctx := context.Background()
	if err := rt.Start(bgctx); err != nil {
		slog.Error("runtime.Start failed", "machine", mid, "err", err)
		http.Error(w, "failed to start runtime", http.StatusInternalServerError)
		return
	}
	currentID := rt.GetCurrentState()
	resp := CreateInstanceResp{
		ID:     fmt.Sprintf("i%d", atomic.AddInt64(&nextInstanceID, 1)),
		Current: aug.StatePathByID[currentID],
	}
	var instMap map[string]*statechartx.Runtime
	if v, ok := instances.Load(mid); ok {
		instMap = v.(map[string]*statechartx.Runtime)
	} else {
		instMap = make(map[string]*statechartx.Runtime)
	}
	instMap[resp.ID] = rt
	instances.Store(mid, instMap)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type SendEventReq struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type SendEventResp struct {
	Current string   `json:"current"`
	History []string `json:"history"`
}

func sendEvent(w http.ResponseWriter, r *http.Request) {
	mid := chi.URLParam(r, "machineID")
	iid := chi.URLParam(r, "instID")
	var evtReq SendEventReq
	if err := json.NewDecoder(r.Body).Decode(&evtReq); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if v, ok := instances.Load(mid); ok {
		instMap := v.(map[string]*statechartx.Runtime)
		rt, ok := instMap[iid]
		if !ok {
			http.Error(w, "instance not found", http.StatusNotFound)
			return
		}
		aug, err := getAugmentedMachine(mid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		eid, ok := aug.EventIDByName[evtReq.Type]
		if !ok {
			http.Error(w, fmt.Sprintf("event type %q not found", evtReq.Type), http.StatusBadRequest)
			return
		}
		evt := statechartx.Event{ID: eid, Data: evtReq.Data}
		rt.ProcessEvent(evt)
		currentID := rt.GetCurrentState()
		resp := SendEventResp{
			Current: aug.StatePathByID[currentID],
			History: []string{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	http.Error(w, "machine instances not found", http.StatusNotFound)
}
