package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/comalice/statechartx"
	registrystatechart "github.com/comalice/maelstrom/registry/statechart"
)

func instancePath(machineID, instID string) string {
	return filepath.Join("instances", machineID, instID+".json")
}

func loadInstanceState(path string) (*InstanceState, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	var state InstanceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return &state, true, nil
}

func saveInstanceState(path string, state *InstanceState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}


func getInstanceMutex(machineID, instID string) *sync.Mutex {
	key := machineID + ":" + instID
	v, _ := instanceMutexes.LoadOrStore(key, new(sync.Mutex))
	return v.(*sync.Mutex)
}

func replayRuntime(rt *statechartx.Runtime, aug *registrystatechart.AugmentedMachine, history []EventLog) error {
	for _, log := range history {
		eid, ok := aug.EventIDByName[log.Type]
		if !ok {
			slog.Warn("replay skip unknown event", "type", log.Type)
			continue
		}
		var data any
		if err := json.Unmarshal(log.Data, &data); err != nil {
			slog.Warn("replay unmarshal data", "type", log.Type, "err", err)
			continue
		}
		evt := statechartx.Event{ID: eid, Data: data}
		rt.ProcessEvent(evt)
	}
	return nil
}