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
	instances         sync.Map // machineID -> map[instID]*statechartx.Runtime
	nextInstanceID    int64
	instanceMutexes   sync.Map // mid:iid -> *sync.Mutex
)

type EventLog struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type InstanceState struct {
	Initial json.RawMessage `json:"initialContext"`
	History []EventLog      `json:"history"`
}

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
	if err := json.NewEncoder(w).Encode(mids); err != nil {
		slog.Error("json encode", "err", err)
	}
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
	iid := fmt.Sprintf("i%d", atomic.AddInt64(&nextInstanceID, 1))
	path := instancePath(mid, iid)
	initialBytes, _ := json.Marshal(req.InitialContext)
	state := &InstanceState{Initial: json.RawMessage(initialBytes), History: []EventLog{}}
	if err := saveInstanceState(path, state); err != nil {
		http.Error(w, fmt.Sprintf("save instance: %v", err), http.StatusInternalServerError)
		return
	}
	bgctx := context.Background()
	rt := statechartx.NewRuntime(aug.Machine, req.InitialContext)
	if err := rt.Start(bgctx); err != nil {
		slog.Error("runtime.Start failed", "machine", mid, "iid", iid, "err", err)
		http.Error(w, "failed to start runtime", http.StatusInternalServerError)
		return
	}
	currentID := rt.GetCurrentState()
	resp := CreateInstanceResp{
		ID: iid,
		Current: aug.StatePathByID[currentID],
	}
	var instMap map[string]*statechartx.Runtime
	if v, ok := instances.Load(mid); ok {
		instMap = v.(map[string]*statechartx.Runtime)
	} else {
		instMap = make(map[string]*statechartx.Runtime)
	}
	instMap[iid] = rt
	instances.Store(mid, instMap)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("json encode", "err", err)
	}
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
	path := instancePath(mid, iid)
	mu := getInstanceMutex(mid, iid)
	mu.Lock()
	defer mu.Unlock()
	state, ok, err := loadInstanceState(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("load instance: %v", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}
	aug, err := getAugmentedMachine(mid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var initialData any
	if err := json.Unmarshal(state.Initial, &initialData); err != nil {
		slog.Error("unmarshal initial", "iid", iid, "err", err)
		initialData = map[string]any{}
	}
	bgctx := context.Background()
	rt := statechartx.NewRuntime(aug.Machine, initialData)
	if err := rt.Start(bgctx); err != nil {
		slog.Error("replay rt.Start failed", "mid", mid, "iid", iid, "err", err)
		http.Error(w, "failed to start runtime", http.StatusInternalServerError)
		return
	}
	if err := replayRuntime(rt, aug, state.History); err != nil {
		slog.Error("replay failed", "mid", mid, "iid", iid, "err", err)
	}
	eid, ok := aug.EventIDByName[evtReq.Type]
	if !ok {
		http.Error(w, fmt.Sprintf("event type %q not found", evtReq.Type), http.StatusBadRequest)
		return
	}
	evt := statechartx.Event{ID: eid, Data: evtReq.Data}
	rt.ProcessEvent(evt)
	evtDataBytes, _ := json.Marshal(evtReq.Data)
	newLog := EventLog{
		Type: evtReq.Type,
		Data: json.RawMessage(evtDataBytes),
	}
	state.History = append(state.History, newLog)
	if err := saveInstanceState(path, state); err != nil {
		slog.Error("save failed", "mid", mid, "iid", iid, "err", err)
		http.Error(w, fmt.Sprintf("save instance: %v", err), http.StatusInternalServerError)
		return
	}
	currentID := rt.GetCurrentState()
	resp := SendEventResp{
		Current: aug.StatePathByID[currentID],
		History: []string{}, // or fmt.Sprintf("%d events", len(state.History))
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("json encode", "err", err)
	}
}
