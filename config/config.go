package config

import (
	"encoding/json"
	"reflect"
	"strings"
)

// ConfigField represents a configuration field for CLI table and JSON output.
type ConfigField struct {
	Field   string `json:"field"`
	Type    string `json:"type"`
	Env     string `json:"env"`
	Desc    string `json:"desc"`
	Default string `json:"default"`
}

// AppConfig holds the server configuration.
//
// Fields are documented with godoc and tags for dynamic CLI/docs generation.
type AppConfig struct {
	// ListenAddr is the address to bind the HTTP server to.
	// Environment: LISTEN_ADDR
	// Default: :8080
	ListenAddr string `envconfig:"LISTEN_ADDR" desc:"Address to bind HTTP server to" default:":8080"`

	// RegistryDir is the directory to watch for YAML registry files.
	// Environment: REGISTRY_DIR
	// Default: ./yaml
	RegistryDir string `envconfig:"REGISTRY_DIR" desc:"Directory for YAML registry files" default:"./yaml"`

	DefaultModel        string         `envconfig:"DEFAULT_MODEL" desc:"Default LLM model" default:"claude-3-5-sonnet-20240620"`
	DefaultProvider     string         `envconfig:"DEFAULT_PROVIDER" desc:"Default LLM provider" default:"anthropic"`
	DefaultBaseURL      *string        `envconfig:"DEFAULT_BASE_URL" desc:"Default LLM base URL"`
	DefaultTemperature  *float64       `envconfig:"DEFAULT_TEMPERATURE" desc:"Default temperature" default:"0.7"`
	DefaultMaxTokens    *int           `envconfig:"DEFAULT_MAX_TOKENS" desc:"Default max tokens" default:"4096"`
	DefaultAPIKey       string         `envconfig:"DEFAULT_API_KEY" desc:"Default API key (or env:VAR)"`
}

// AppConfigFields returns slice of ConfigField from AppConfig struct tags via reflect.
func AppConfigFields() []ConfigField {
	t := reflect.TypeOf(AppConfig{})
	n := t.NumField()
	fields := make([]ConfigField, 0, n)
	for i := 0; i < n; i++ {
		f := t.Field(i)
		envTag := f.Tag.Get("envconfig")
		env := strings.Trim(envTag, `"`)
		desc := f.Tag.Get("desc")
		def := strings.Trim(f.Tag.Get("default"), `"`)
		fields = append(fields, ConfigField{
			Field:   f.Name,
			Type:    f.Type.Kind().String(),
			Env:     env,
			Desc:    desc,
			Default: def,
		})
	}
	return fields
}

// AppConfigSchema returns dynamic OpenAPI 3.0 schema as JSON bytes for Swagger UI.
func AppConfigSchema() []byte {
	fields := AppConfigFields()
	props := map[string]map[string]interface{}{}
	for _, f := range fields {
		propName := strings.ToLower(f.Field[:1]) + f.Field[1:]
		p := map[string]interface{}{
			"type":        "string",
			"description": f.Desc,
		}
		if f.Default != "" {
			p["default"] = f.Default
		}
		props[propName] = p
	}
	appConfigSchema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	paths := map[string]interface{}{
		"/config": map[string]interface{}{
			"get": map[string]interface{}{
				"summary": "Configuration schema",
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "AppConfig OpenAPI schema",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AppConfig",
								},
							},
						},
					},
				},
			},
		},
	}
	openapi := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Maelstrom Configuration",
			"description": "Interactive docs for environment configuration fields.",
			"version":     "1.0.0",
		},
		"paths": paths,
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"AppConfig": appConfigSchema,
			},
		},
	}
	b, _ := json.MarshalIndent(openapi, "", "  ")
	return b
}
