package yaml

import (
	"os"
	"path/filepath"
	"testing"
	"github.com/comalice/maelstrom/config"
	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
	assert.Equal(t, content, raw)
	assert.Equal(t, "1.0", ver)
}

func TestRenderPlain(t *testing.T) {
	raw := `key: value`
	content, err := Render(raw, nil)
	assert.NoError(t, err)
	assert.Equal(t, "value", content["key"].(string))
}

func TestRenderAppFoo(t *testing.T) {
	raw := `foo: {{.Env.FOO}}`
	type data struct {
		Env map[string]string `json:"-"` // Mimic Variables (trimmed)
	}
	d := data{Env: map[string]string{"FOO": "bar"}} // Raw pass-through test
	content, err := Render(raw, d)
	assert.NoError(t, err)
	assert.Equal(t, "bar", content["foo"].(string))
}


func TestRenderInvalidTemplate(t *testing.T) {
	raw := `{{ invalid syntax }}`
	_, err := Render(raw, nil)
	assert.Error(t, err)
}

func TestTemplateVariableResolution(t *testing.T) {
	raw := `env: {{.App.Environment}}
company: {{.App.CompanyName}}
var: {{.Env.VAR_NAME}}`
	cfg := &config.AppConfig{Environment: "prod", CompanyName: "Acme", Variables: map[string]string{"VAR_NAME": "value"}}
	rd := struct{App *config.AppConfig; Env map[string]string}{App: cfg, Env: cfg.Variables}
	content, err := Render(raw, rd)
	assert.NoError(t, err)
	assert.Equal(t, "prod", content["env"])
	assert.Equal(t, "Acme", content["company"])
	assert.Equal(t, "value", content["var"])
}

func TestEnvironmentVariableAccess(t *testing.T) {
	raw := `env: {{.App.Environment}}`
	rd := struct{App *config.AppConfig; Env map[string]string}{App: &config.AppConfig{Environment: "prod"}, Env: map[string]string{}}
	content, err := Render(raw, rd)
	assert.NoError(t, err)
	assert.Equal(t, "prod", content["env"])
}

func TestNestedVariableAccess(t *testing.T) {
	_, err := Render(`{{.Env.FOO.BAR}}`, struct{App *config.AppConfig; Env map[string]string}{Env: map[string]string{}})
	assert.Error(t, err)  // No nesting support
}
