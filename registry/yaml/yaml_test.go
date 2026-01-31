package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRawParseFile(t *testing.T) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-app-v1.0.yaml")
	content := `key: value`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	raw, ver, err := RawParseFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if raw != content {
		t.Errorf("expected raw %q, got %q", content, raw)
	}
	if ver != "1.0" {
		t.Errorf("expected ver 1.0, got %q", ver)
	}
}

func TestRenderPlain(t *testing.T) {
	raw := `key: value`
	content, err := Render(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := content["key"].(string); !ok || v != "value" {
		t.Errorf("expected {'key':'value'}, got %v", content)
	}
}

func TestRenderTemplate(t *testing.T) {
	raw := `dir: {{ .Config.RegistryDir }}
foo: {{ .Env.FOO }}`
	type data struct {
		Config *struct{ RegistryDir string }
		Env    map[string]string
	}
	d := data{
		Config: &struct{ RegistryDir string }{"/test"},
		Env:    map[string]string{"FOO": "baz"},
	}
	content, err := Render(raw, d)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := content["dir"].(string); !ok || v != "/test" {
		t.Errorf("expected dir /test, got %v", content["dir"])
	}
	if v, ok := content["foo"].(string); !ok || v != "baz" {
		t.Errorf("expected foo baz, got %v", content["foo"])
	}
}

func TestRenderInvalidTemplate(t *testing.T) {
	raw := `{{ invalid syntax }}`
	_, err := Render(raw, nil)
	if err == nil {
		t.Error("expected parse error")
	}
}
