package registry

import (
	"os"
	"testing"

	"github.com/comalice/maelstrom/config"
	"github.com/stretchr/testify/assert"
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

func TestSetConfig_Resolver(t *testing.T) {
	r := New()
	cfg := &config.AppConfig{
		DefaultModel: "test-model",
	}
	r.SetConfig(cfg)
	assert.NotNil(t, r.resolver)
	assert.NotNil(t, r.Config)
}

func TestList_Resolves(t *testing.T) {
	r := New()
	cfg := &config.AppConfig{
		DefaultModel:    "app-model",
		DefaultProvider: "app-provider",
		DefaultAPIKey:   "app-key",
	}
	r.SetConfig(cfg)

	r.items = map[string]*YAMLImport{
		"test.yaml": {
			Raw:     `llm:
  model: yaml-model
  provider: yaml-provider
  api_key: yaml-key
  base_url: https://api.example.com
  temperature: 0.8
  max_tokens: 8192
  tool_policies:
    - policy1
    - policy2`,
			Version: "1.0",
			Active:  true,
		},
		"no-llm.yaml": {
			Raw:     `name: no-llm`,
			Version: "1.0",
			Active:  true,
		},
	}

	list := r.List()
	assert.Len(t, list, 2)

	// Test with llm
	testItem := list[0]
	assert.True(t, testItem.Active)
	content := testItem.Content
	assert.NotNil(t, content["llm"])
	resolved := content["resolved"].(map[string]interface{})
	assert.Equal(t, "yaml-model", resolved["model"])
	assert.Equal(t, "yaml-provider", resolved["provider"])
	assert.Equal(t, "yaml-key", resolved["api_key"])
	assert.Equal(t, "https://api.example.com", *resolved["base_url"].(*string))
	assert.Equal(t, 0.8, *resolved["temperature"].(*float64))
	assert.Equal(t, 8192, *resolved["max_tokens"].(*int))
	assert.Equal(t, []string{"policy1", "policy2"}, resolved["tool_policies"].([]string))

	// Test no llm - defaults
	noLlmItem := list[1]
	resolvedNoLlm := noLlmItem.Content["resolved"].(map[string]interface{})
	assert.Equal(t, "app-model", resolvedNoLlm["model"])
	assert.Equal(t, "app-provider", resolvedNoLlm["provider"])
	assert.Equal(t, "app-key", resolvedNoLlm["api_key"])
}