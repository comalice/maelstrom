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

func TestRenderAppFoo(t *testing.T) {
	raw := `foo: {{ .Env.APP_FOO }}`
	type data struct {
		Env map[string]string `json:"-"` // Mimic Variables (trimmed)
	}
	d := data{Env: map[string]string{"APP_FOO": "bar"}} // Raw pass-through test
	content, err := Render(raw, d)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := content["foo"].(string); !ok || v != "bar" {
		t.Errorf("expected foo bar (APP_FOO passthru), got %v", content["foo"])
	}
}


func TestRenderInvalidTemplate(t *testing.T) {
	raw := `{{ invalid syntax }}`
	_, err := Render(raw, nil)
	if err == nil {
		t.Error("expected parse error")
	}
}
