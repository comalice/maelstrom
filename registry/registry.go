package registry

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/comalice/maelstrom/config"
	"github.com/comalice/maelstrom/registry/statechart"
	"github.com/comalice/maelstrom/registry/yaml"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/comalice/maelstrom/internal/llm"
	yamlv3 "gopkg.in/yaml.v3"
)

type YAMLImport struct {
	Content             map[string]any       `json:"content"`
	Version             string              `json:"version"`
	Active              bool                `json:"active"`
	Filename            string              `json:"filename"`
	Type                string              `json:"type,omitempty"`
	StatechartAugmented *statechart.AugmentedMachine `json:"-"`
	Raw                 string              `json:"-"`
}

type RawYAML struct {
	Raw     string `json:"raw"`
	Version string `json:"version"`
	Active  bool   `json:"active"`
}

type Registry struct {
	mu       sync.RWMutex
	items    map[string]*YAMLImport
	watcher  *fsnotify.Watcher
	stop     chan struct{}
	dir      string                          // Track watch dir
	Config   *config.AppConfig               `json:"-"`
	Env      map[string]string               `json:"-"`
	resolver *config.ConfigHierarchyResolver `json:"-"`
	AgentsDir    string                           `json:"agents_dir"`
	RuntimeDir   string                           `json:"runtime_dir"`
	MaxAgents    int                              `json:"max_agents"`
	CostPerHour  float64                          `json:"cost_per_hour"`
	NumAgents      atomic.Int32                     `json:"num_agents"`
	MaxLLMCalls    atomic.Int32                     `json:"max_llm_calls"`
	Machines       map[string]*statechart.AugmentedMachine `json:"-"`
}

var ErrMaxAgents = errors.New("max agents reached")
var ErrMaxLLMCalls = errors.New("max llm calls reached")

var GlobalRegistry *Registry

func New() *Registry {
	return &Registry{
		items:    make(map[string]*YAMLImport),
		Machines: make(map[string]*statechart.AugmentedMachine),
		stop:     make(chan struct{}),
		MaxAgents: 5,
		NumAgents: atomic.Int32{},
	}
}

func (r *Registry) SetDir(dir string) { r.dir = dir }

func (r *Registry) SetConfig(cfg *config.AppConfig) {
	r.Config = cfg
	slog.Info("registry config set")
	r.resolver = config.NewResolver(r.Config)
}

func (r *Registry) scanDir() error {
	files, err := filepath.Glob(filepath.Join(r.dir, "*.{yaml,yml}"))
	if err != nil {
		return err
	}
	for _, f := range files {
		name := filepath.Base(f)
		if err := r.Import(name); err != nil {
			slog.Warn("initial import failed", "file", name, "err", err)
		} else {
			slog.Info("initial import", "file", name)
		}
	}
	return nil
}

func (r *Registry) InitWatcher(dir string) error {
	r.dir = dir
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return err
	}
	if err := r.scanDir(); err != nil {
		slog.Warn("scan dir failed", "err", err)
	}
	r.watcher = w
	GlobalRegistry = r
	go r.watch()
	return nil
}

func (r *Registry) watch() {
	defer r.watcher.Close()
	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			name := filepath.Base(event.Name)
			matchYAML, _ := filepath.Match("*.yaml", name)
			matchYML, _ := filepath.Match("*.yml", name)
			if !matchYAML && !matchYML {
				continue
			}
			if event.Op&fsnotify.Create != 0 || event.Op&fsnotify.Write != 0 {
				raw, ver, err := yaml.RawParseFile(event.Name)
				if err != nil {
					slog.Error("raw parse failed", "file", event.Name, "err", err)
					continue
				}
				r.mu.Lock()
				r.items[name] = &YAMLImport{Raw: raw, Version: ver, Active: true, Filename: name}
				r.mu.Unlock()
				slog.Info("imported raw", "file", name, "ver", ver)
			} else if event.Op&fsnotify.Remove != 0 || event.Op&fsnotify.Rename != 0 {
				r.mu.Lock()
				if imp, ok := r.items[name]; ok {
					imp.Active = false
					slog.Info("deactivated", "file", name)
				}
				r.mu.Unlock()
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher err", "err", err)
		case <-r.stop:
			return
		}
	}
}

func (r *Registry) Stop() {
	close(r.stop)
}

func (r *Registry) List() []*YAMLImport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*YAMLImport, 0, len(r.items))
	for _, item := range r.items {
		newItem := &YAMLImport{
			Version:  item.Version,
			Active:   item.Active,
			Filename: item.Filename,
			Raw:      item.Raw,
			Content:  map[string]any{},
		}
		var renderErr error
		if newItem.Raw != "" {
			if r.Config != nil {
				type renderData struct {
					App *config.AppConfig `json:"-"`
					Env    map[string]string `json:"-"`
				}
				data := renderData{
					App: r.Config,
					Env: r.Config.Variables,
				}
				newItem.Content, renderErr = yaml.Render(newItem.Raw, data)
				if renderErr != nil {
					slog.Warn("render failed", "file", item.Version, "err", renderErr)
					newItem.Content = map[string]any{}
				}
			} else {
				if err := yamlv3.Unmarshal([]byte(newItem.Raw), &newItem.Content); err != nil {
					newItem.Content = map[string]any{}
				}
			}
		}

		if r.resolver != nil && newItem.Content != nil && len(newItem.Content) > 0 {
			res := r.resolver.Resolve(newItem.Content, nil, nil)
			newItem.Content["resolved"] = config.ToResolvedMap(res)
		}

		// Attempt to parse as statechart
		var parseBytes []byte
		if renderErr == nil {
			renderedBytes, _ := yamlv3.Marshal(newItem.Content)
			parseBytes = renderedBytes
		} else {
			parseBytes = []byte(newItem.Raw)
		}
		spec, perr := statechart.ParseSpec(parseBytes)
		if perr == nil && spec.Machine.ID != "" {
			newItem.Type = "statechart"
			if r.resolver != nil {
				resolved := r.resolver.Resolve(newItem.Content, nil, nil)
				spec.LLM = toLLMConfig(resolved)
			}
			aug, merr := spec.ToAugmentedMachine(r)
			if merr == nil {
				newItem.StatechartAugmented = aug
			} else {
				slog.Warn("statechart ToAugmentedMachine failed", "file", newItem.Filename, "err", merr)
			}
		} else {
			newItem.Type = "yaml"
		}
		list = append(list, newItem)
	}
	return list
}

func (r *Registry) ListRaw() []RawYAML {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]RawYAML, 0, len(r.items))
	for _, item := range r.items {
		list = append(list, RawYAML{
			Raw:     item.Raw,
			Version: item.Version,
			Active:  item.Active,
		})
	}
	return list
}

func toLLMConfig(res *config.ResolvedMachineConfig) llm.LLMConfig {
	endpoint := ""
	if res.BaseURL != nil {
		endpoint = *res.BaseURL
	}
	temp := float64(0.7)
	if res.Temperature != nil {
		temp = *res.Temperature
	}
	tokens := 4096
	if res.MaxTokens != nil {
		tokens = *res.MaxTokens
	}
	return llm.LLMConfig{
		Provider:   res.Provider,
		Model:      res.Model,
		APIKey:     res.APIKey,
		Endpoint:   endpoint,
		Temp:       temp,
		MaxTokens:  tokens,
	}
}

func (r *Registry) HireAgent(template string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if int(r.NumAgents.Load()) >= r.MaxAgents {
		return ErrMaxAgents
	}

	if r.AgentsDir == "" {
		return fmt.Errorf("AgentsDir not set")
	}

	agentPath := filepath.Join(r.AgentsDir, template+".yaml")
	dataBytes, err := os.ReadFile(agentPath)
	if err != nil {
		return fmt.Errorf("read agent template %q: %w", agentPath, err)
	}

	agentRaw := string(dataBytes)

	type renderData struct {
		App *config.AppConfig `json:"-"`
		Env map[string]string `json:"-"`
	}
	data := renderData{App: r.Config, Env: r.Config.Variables}
	content, renderErr := yaml.Render(agentRaw, data)

	var parseBytes []byte
	if renderErr == nil {
		renderedBytes, _ := yamlv3.Marshal(content)
		parseBytes = renderedBytes
		slog.Info("agent template rendered", "template", template)
	} else {
		parseBytes = []byte(agentRaw)
		slog.Warn("agent template render failed, using raw", "template", template, "err", renderErr)
	}

	spec, err := statechart.ParseSpec(parseBytes)
	if err != nil {
		return fmt.Errorf("parse agent spec %q: %w", template, err)
	}

	aug, err := spec.ToAugmentedMachine(r)
	if err != nil {
		return fmt.Errorf("augment agent machine %q: %w", template, err)
	}

	id := uuid.New().String()
	r.Machines[id] = aug
	r.NumAgents.Add(1)

	slog.Info("hired agent", "id", id, "template", template)
	return nil
}

func (r *Registry) RetireAgent(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Machines[id]; ok {
		delete(r.Machines, id)
		r.NumAgents.Add(-1)
		slog.Info("retired agent", "id", id)
	} else {
		return fmt.Errorf("agent %q not found", id)
	}
	return nil
}

func (r *Registry) SendMessage(toID string, msg map[string]any) error {
	r.mu.RLock()
	_, ok := r.Machines[toID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %q not found", toID)
	}
	// Stub: log message, dispatch event later
	slog.Info("SendMessage stubbed", "toID", toID, "msg", msg)
	return nil
}

func (r *Registry) QueryAgents() map[string]statechart.AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]statechart.AgentInfo)
	for id, aug := range r.Machines {
		m[id] = statechart.AgentInfo{
			ID:      id,
			Current: aug.Current(),
			History: aug.History(),
		}
	}
	return m
}

func (r *Registry) Import(filename string) error {
	full := filepath.Join(r.dir, filename)
	raw, ver, err := yaml.RawParseFile(full)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.items[filename] = &YAMLImport{Raw: raw, Version: ver, Active: true, Filename: filename}
	r.mu.Unlock()
	slog.Info("manual import raw", "file", filename, "ver", ver)
	return nil
}
