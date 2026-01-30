package registry

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/comalice/maelstrom/registry/yaml"
	"github.com/fsnotify/fsnotify"
)

type YAMLImport struct {
	Content map[string]interface{} `json:"content"`
	Version string                 `json:"version"`
	Active  bool                   `json:"active"`
}

type Registry struct {
	mu      sync.RWMutex
	items   map[string]*YAMLImport
	watcher *fsnotify.Watcher
	stop    chan struct{}
	dir     string // Track watch dir
}

var GlobalRegistry *Registry

func New() *Registry {
	return &Registry{
		items: make(map[string]*YAMLImport),
		stop:  make(chan struct{}),
	}
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
				content, ver, err := yaml.ParseFile(event.Name)
				if err != nil {
					slog.Error("parse failed", "file", event.Name, "err", err)
					continue
				}
				r.mu.Lock()
				r.items[name] = &YAMLImport{Content: content, Version: ver, Active: true}
				r.mu.Unlock()
				slog.Info("imported", "file", name, "ver", ver)
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
		list = append(list, item)
	}
	return list
}

func (r *Registry) Import(filename string) error {
	full := filepath.Join(r.dir, filename)
	content, ver, err := yaml.ParseFile(full)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.items[filename] = &YAMLImport{Content: content, Version: ver, Active: true}
	r.mu.Unlock()
	slog.Info("manual import", "file", filename, "ver", ver)
	return nil
}
