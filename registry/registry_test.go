package registry

import (
	"os"
	"testing"

	"github.com/comalice/maelstrom/config"
)

func TestSetConfig(t *testing.T) {
	r := New()
	cfg := &config.AppConfig{RegistryDir: "/test"}
	os.Clearenv() // Reset env
	os.Setenv("FOO", "baz")
	r.SetConfig(cfg)
	if r.Config.RegistryDir != "/test" {
		t.Errorf("expected Config.RegistryDir /test")
	}
	if v, ok := r.Env["FOO"]; !ok || v != "baz" {
		t.Errorf("expected Env.FOO=baz")
	}
}

func TestListRaw(t *testing.T) {
	r := New()
	r.items = map[string]*YAMLImport{
		"test.yaml": {Raw: "key: value", Version: "1.0", Active: true},
	}
	list := r.ListRaw()
	if len(list) != 1 || list[0].Raw != "key: value" {
		t.Errorf("expected raw list")
	}
}

func TestListRendered(t *testing.T) {
	r := New()
	r.Config = &config.AppConfig{RegistryDir: "/testdir"}
	r.Env = map[string]string{"FOO": "baz"}
	r.items = map[string]*YAMLImport{
		"test.yaml": {Raw: `dir: {{ .Config.RegistryDir }}
foo: {{ .Env.FOO }}`, Version: "1.0", Active: true},
	}
	list := r.List()
	if len(list) != 1 {
		t.Fatal("expected 1 item")
	}
	item := list[0]
	if dir, ok := item.Content["dir"].(string); !ok || dir != "/testdir" {
		t.Errorf("expected dir /testdir, got %v", item.Content["dir"])
	}
	if foo, ok := item.Content["foo"].(string); !ok || foo != "baz" {
		t.Errorf("expected foo baz, got %v", item.Content["foo"])
	}
}
