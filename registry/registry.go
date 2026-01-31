package registry

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/comalice/maelstrom/config"
	"github.com/comalice/maelstrom/registry/yaml"
	yamlv3 "gopkg.in/yaml.v3"
	"github.com/fsnotify/fsnotify"
)

type YAMLImport struct {
	Content map[string]interface{} `json:"content"`
	Version string                 `json:"version"`
	Active  bool                   `json:"active"`
	Raw     string                 `json:"-"`
}

type RawYAML struct {
	Raw    string `json:"raw"`
	Version string `json:"version"`
	Active bool   `json:"active"`
}

type Registry struct {
	mu      sync.RWMutex
	items   map[string]*YAMLImport
	watcher *fsnotify.Watcher
	stop    chan struct{}
	dir     string // Track watch dir
	Config  *config.AppConfig `json:"-"`
	Env     map[string]string `json:"-"`
}

var GlobalRegistry *Registry

func New() *Registry {
	return &Registry{
		items: make(map[string]*YAMLImport),
		stop:  make(chan struct{}),
	}
}

func (r *Registry) SetConfig(cfg *config.AppConfig) {
	r.Config = cfg
	r.Env = make(map[string]string)
	for _, pair := range os.Environ() {
		if i := strings.Index(pair, "="); i > 0 {
			r.Env[pair[:i]] = pair[i+1:]
		}
	}
	slog.Info("registry config and env set")
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
				r.items[name] = &YAMLImport{Raw: raw, Version: ver, Active: true}
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
			Version: item.Version,
			Active:  item.Active,
			Raw:     item.Raw,
		}
		if newItem.Raw != "" {
			if r.Config != nil {
				type renderData struct {
					Config *config.AppConfig  `json:"-"`
					Env    map[string]string `json:"-"`
				}
				var data renderData
				data.Config = r.Config
				data.Env = r.Env
				var renderErr error
				newItem.Content, renderErr = yaml.Render(newItem.Raw, data)
				if renderErr != nil {
					slog.Warn("render failed", "file", item.Version, "err", renderErr)
					newItem.Content = map[string]interface{}{}
				}
			} else {
				if err := yamlv3.Unmarshal([]byte(newItem.Raw), &newItem.Content); err != nil {
					newItem.Content = map[string]interface{}{}
				}
			}
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
			Raw:    item.Raw,
			Version: item.Version,
			Active: item.Active,
		})
	}
	return list
}

func (r *Registry) Import(filename string) error {
	full := filepath.Join(r.dir, filename)
	raw, ver, err := yaml.RawParseFile(full)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.items[filename] = &YAMLImport{Raw: raw, Version: ver, Active: true}
	r.mu.Unlock()
	slog.Info("manual import raw", "file", filename, "ver", ver)
	return nil
}
