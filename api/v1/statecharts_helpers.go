package v1

import (
	"encoding/json"
	"errors"
	"fmt"
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := filepath.Join(dir, "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) // cleanup
		return fmt.Errorf("rename %s to %s: %w", tmp, path, err)
	}
	return nil
}

func deleteInstanceState(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
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
			return fmt.Errorf("replay unknown event %q", log.Type)
		}
		var data any
		if err := json.Unmarshal(log.Data, &data); err != nil {
			return fmt.Errorf("replay unmarshal data %s: %w", log.Type, err)
		}
		evt := statechartx.Event{ID: eid, Data: data}
		rt.ProcessEvent(evt)
	}
	return nil
}