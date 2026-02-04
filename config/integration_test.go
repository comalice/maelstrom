package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/comalice/maelstrom/config"
	"github.com/comalice/maelstrom/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func floatp(f float64) *float64 {
	p := f
	return &p
}

func intp(i int) *int {
	p := i
	return &p
}

func TestIntegration_ListRenderedResolvedStatechart(t *testing.T) {
	tests := []struct {
		name              string
		yamlContent       string
		appConfig         *config.AppConfig
		testEnv           map[string]string
		expectedContent   map[string]any
		expectedResolved  map[string]any
		expectedType      string
		expectedMachineID string
	}{
		{
			name:        "plain_yaml_defaults",
			yamlContent: `key: value`,
			appConfig: &config.AppConfig{
				DefaultModel:       "app-model",
				DefaultProvider:    "app-prov",
				DefaultAPIKey:      "app-key",
				DefaultTemperature: floatp(0.7),
				DefaultMaxTokens:   intp(4096),
			},
			testEnv: nil,
			expectedContent: map[string]any{
				"key": "value",
				"resolved": map[string]any{
					"model":           "app-model",
					"provider":        "app-prov",
					"api_key":         "app-key",
					"base_url":        (*string)(nil),
					"temperature":     floatp(0.7),
					"max_tokens":      intp(4096),
					"tool_policies":   ([]string)(nil),
					"allowed_actions": ([]string)(nil),
				},
			},
			expectedResolved: map[string]any{
				"model":           "app-model",
				"provider":        "app-prov",
				"api_key":         "app-key",
				"base_url":        (*string)(nil),
				"temperature":     floatp(0.7),
				"max_tokens":      intp(4096),
				"tool_policies":   ([]string)(nil),
				"allowed_actions": ([]string)(nil),
			},
			expectedType:      "yaml",
			expectedMachineID: "",
		},
		{
			name: "statechart_parse",
			yamlContent: `name: test-sc
version: 1.0
machine:
  id: root
  initial: idle
  states:
    idle: {}`,
			appConfig: &config.AppConfig{
				DefaultModel:       "sc-default",
				DefaultProvider:    "anthropic",
				DefaultTemperature: floatp(0.7),
				DefaultMaxTokens:   intp(4096),
			},
			testEnv: nil,
			expectedContent: map[string]any{
				"name":    "test-sc",
				"version": 1.0,
				"machine": map[string]any{
					"id":      "root",
					"initial": "idle",
					"states": map[string]any{
						"idle": map[string]any{},
					},
				},
				"resolved": map[string]any{
					"model":           "sc-default",
					"provider":        "anthropic",
					"api_key":         "",
					"base_url":        (*string)(nil),
					"temperature":     floatp(0.7),
					"max_tokens":      intp(4096),
					"tool_policies":   ([]string)(nil),
					"allowed_actions": ([]string)(nil),
				},
			},
			expectedResolved: map[string]any{
				"model":           "sc-default",
				"provider":        "anthropic",
				"api_key":         "",
				"base_url":        (*string)(nil),
				"temperature":     floatp(0.7),
				"max_tokens":      intp(4096),
				"tool_policies":   ([]string)(nil),
				"allowed_actions": ([]string)(nil),
			},
			expectedType:      "statechart",
			expectedMachineID: "root",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.testEnv != nil {
				for k, v := range tt.testEnv {
					os.Setenv(k, v)
				}
			}
			dir := t.TempDir()
			filename := "test.yaml"
			fullpath := filepath.Join(dir, filename)
			require.NoError(t, os.WriteFile(fullpath, []byte(tt.yamlContent), 0644))
			r := registry.New()
			r.SetDir(dir)
			r.SetConfig(tt.appConfig)
			require.NoError(t, r.Import(filename))
			list := r.List()
			require.Len(t, list, 1)
			item := list[0]
			assert.True(t, item.Active)
			assert.Equal(t, filename, item.Filename)
			assert.Equal(t, tt.expectedContent, item.Content)
			resIface, ok := item.Content["resolved"]
			require.True(t, ok)
			resolved := resIface.(map[string]any)
			assert.Equal(t, tt.expectedResolved, resolved)
			assert.Equal(t, tt.expectedType, item.Type)
			if tt.expectedMachineID != "" {
				require.NotNil(t, item.StatechartAugmented)
				assert.Equal(t, tt.expectedMachineID, item.StatechartAugmented.Spec.Machine.ID)
			} else {
				assert.Nil(t, item.StatechartAugmented)
			}
		})
	}
}
