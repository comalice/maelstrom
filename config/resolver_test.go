package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResolver(t *testing.T) {
	cfg := &AppConfig{}
	resolver := NewResolver(cfg)
	assert.NotNil(t, resolver)
	assert.Equal(t, cfg, resolver.cfg)
}

func TestResolve_Hierarchy(t *testing.T) {
	tests := []struct {
		name      string
		appCfg    *AppConfig
		machine   map[string]interface{}
		action    map[string]interface{}
		guard     map[string]interface{}
		wantModel string
	}{
		{
			name:      "action overrides all",
			appCfg:    &AppConfig{DefaultModel: "app-model", DefaultProvider: "app-provider"},
			machine:   map[string]interface{}{"llm": map[string]interface{}{"model": "machine"}},
			action:    map[string]interface{}{"llm": map[string]interface{}{"model": "action"}},
			guard:     nil,
			wantModel: "action",
		},
		{
			name:      "machine overrides app",
			appCfg:    &AppConfig{DefaultModel: "app-model"},
			machine:   map[string]interface{}{"llm": map[string]interface{}{"model": "machine"}},
			action:    nil,
			guard:     nil,
			wantModel: "machine",
		},
		{
			name:      "app default",
			appCfg:    &AppConfig{DefaultModel: "app-model"},
			machine:   nil,
			action:    nil,
			guard:     nil,
			wantModel: "app-model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(tt.appCfg)
			res := r.Resolve(tt.machine, tt.action, tt.guard)
			assert.Equal(t, tt.wantModel, res.Model)
		})
	}
}

func TestResolve_Ptrs(t *testing.T) {
	appCfg := &AppConfig{
		DefaultBaseURL:     strPtr("app-base"),
		DefaultTemperature: floatPtr(0.7),
		DefaultMaxTokens:   intPtr(4096),
	}
	tests := []struct {
		name     string
		machine  map[string]interface{}
		action   map[string]interface{}
		guard    map[string]interface{}
		wantBase *string
	}{
		{
			name:     "ptr from action",
			action:   map[string]interface{}{"llm": map[string]interface{}{"base_url": "action-base"}},
			wantBase: strPtr("action-base"),
		},
		{
			name:     "app default ptr",
			wantBase: appCfg.DefaultBaseURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(appCfg)
			res := r.Resolve(nil, tt.action, tt.guard)
			assert.Equal(t, tt.wantBase, res.BaseURL)
		})
	}
}

func TestResolveAPIKey_Env(t *testing.T) {
	os.Setenv("TEST_KEY", "secret")
	defer os.Unsetenv("TEST_KEY")
	appCfg := &AppConfig{DefaultAPIKey: "env:TEST_KEY"}
	r := NewResolver(appCfg)
	res := r.Resolve(nil, nil, nil)
	assert.Equal(t, "secret", res.APIKey)
}

func TestResolveAPIKey_Direct(t *testing.T) {
	appCfg := &AppConfig{DefaultAPIKey: "direct-key"}
	r := NewResolver(appCfg)
	res := r.Resolve(map[string]interface{}{"llm": map[string]interface{}{"api_key": "override"}}, nil, nil)
	assert.Equal(t, "override", res.APIKey)
}

func TestToResolvedMap(t *testing.T) {
	res := &ResolvedMachineConfig{
		Model:          "model",
		Provider:       "provider",
		BaseURL:        strPtr("base"),
		APIKey:         "key",
		Temperature:    floatPtr(0.7),
		MaxTokens:      intPtr(4096),
		ToolPolicies:   []string{"policy1"},
		AllowedActions: []string{"action1"},
	}
	m := ToResolvedMap(res)
	assert.Equal(t, "model", m["model"])
	assert.Equal(t, "provider", m["provider"])
	assert.Equal(t, strPtr("base"), m["base_url"])
	assert.Equal(t, "key", m["api_key"])
	assert.Equal(t, floatPtr(0.7), m["temperature"])
	assert.Equal(t, intPtr(4096), m["max_tokens"])
	assert.Equal(t, []string{"policy1"}, m["tool_policies"])
}

func strPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func TestHelpers_getString(t *testing.T) {
	r := NewResolver(&AppConfig{DefaultModel: "default"})
	assert.Equal(t, "action", r.getString(map[string]interface{}{"llm": map[string]interface{}{"model": "action"}}, nil, nil, "model", "default"))
}

func TestHelpers_getStringPtr(t *testing.T) {
	r := NewResolver(&AppConfig{})
	ptr := r.getStringPtr(map[string]interface{}{"llm": map[string]interface{}{"base_url": "val"}}, nil, nil, "base_url")
	assert.Equal(t, "val", *ptr)
}

func TestHelpers_getFloatPtr(t *testing.T) {
	r := NewResolver(&AppConfig{})
	ptr := r.getFloatPtr(map[string]interface{}{"llm": map[string]interface{}{"temperature": "0.5"}}, nil, nil, "temperature")
	assert.Equal(t, 0.5, *ptr)
	ptr2 := r.getFloatPtr(map[string]interface{}{"llm": map[string]interface{}{"temperature": 0.5}}, nil, nil, "temperature")
	assert.Equal(t, 0.5, *ptr2)
}

func TestHelpers_getIntPtr(t *testing.T) {
	r := NewResolver(&AppConfig{})
	ptr := r.getIntPtr(map[string]interface{}{"llm": map[string]interface{}{"max_tokens": "4096"}}, nil, nil, "max_tokens")
	assert.Equal(t, 4096, *ptr)
	ptr2 := r.getIntPtr(map[string]interface{}{"llm": map[string]interface{}{"max_tokens": 4096.0}}, nil, nil, "max_tokens")
	assert.Equal(t, 4096, *ptr2)
}

func TestResolve_EmptyMaps(t *testing.T) {
	r := NewResolver(&AppConfig{
		DefaultModel:    "default",
		DefaultProvider: "default",
		DefaultAPIKey:   "default",
	})
	res := r.Resolve(map[string]interface{}{}, nil, nil)
	assert.Equal(t, "default", res.Model)
	assert.Equal(t, "default", res.Provider)
	assert.Equal(t, "default", res.APIKey)
}

func TestGetStringSlice(t *testing.T) {
	r := NewResolver(&AppConfig{})
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want []string
	}{
		{
			name: "valid strings",
			m:    map[string]interface{}{"tool_policies": []interface{}{"pol1", "pol2"}},
			key:  "tool_policies",
			want: []string{"pol1", "pol2"},
		},
		{
			name: "mixed types",
			m:    map[string]interface{}{"tool_policies": []interface{}{"pol1", 123, "pol2"}},
			key:  "tool_policies",
			want: []string{"pol1", "pol2"},
		},
		{
			name: "empty slice",
			m:    map[string]interface{}{"tool_policies": []interface{}{}},
			key:  "tool_policies",
			want: []string{},
		},
		{
			name: "non-slice",
			m:    map[string]interface{}{"tool_policies": "not slice"},
			key:  "tool_policies",
			want: nil,
		},
		{
			name: "missing key",
			m:    map[string]interface{}{},
			key:  "tool_policies",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.getStringSlice(tt.m, tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetIntPtr(t *testing.T) {
	r := NewResolver(&AppConfig{})
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want *int
	}{
		{
			name: "string parse",
			m:    map[string]interface{}{"max_tokens": "4096"},
			key:  "max_tokens",
			want: intPtr(4096),
		},
		{
			name: "float64",
			m:    map[string]interface{}{"max_tokens": 4096.0},
			key:  "max_tokens",
			want: intPtr(4096),
		},
		{
			name: "int",
			m:    map[string]interface{}{"max_tokens": 4096},
			key:  "max_tokens",
			want: intPtr(4096),
		},
		{
			name: "int64",
			m:    map[string]interface{}{"max_tokens": int64(4096)},
			key:  "max_tokens",
			want: intPtr(4096),
		},
		{
			name: "invalid string",
			m:    map[string]interface{}{"max_tokens": "abc"},
			key:  "max_tokens",
			want: nil,
		},
		{
			name: "empty string",
			m:    map[string]interface{}{"max_tokens": ""},
			key:  "max_tokens",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.getIntPtr(tt.m, nil, nil, tt.key)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, *tt.want, *got)
			}
		})
	}
}

func TestResolve_GuardsHierarchy(t *testing.T) {
	tests := []struct {
		name      string
		appCfg    *AppConfig
		machine   map[string]interface{}
		action    map[string]interface{}
		guard     map[string]interface{}
		wantModel string
		wantTemp  *float64
		wantSlices []string
	}{
		{
			name:      "guard fallback",
			appCfg:    &AppConfig{DefaultModel: "app"},
			machine:   nil,
			action:    nil,
			guard:     map[string]interface{}{"llm": map[string]interface{}{"model": "guard"}},
			wantModel: "guard",
		},
		{
			name:      "action overrides guard",
			appCfg:    &AppConfig{},
			machine:   nil,
			action:    map[string]interface{}{"llm": map[string]interface{}{"model": "action"}},
			guard:     map[string]interface{}{"llm": map[string]interface{}{"model": "guard"}},
			wantModel: "action",
		},
		{
			name:      "machine overrides guard ptr",
			appCfg:    &AppConfig{DefaultTemperature: floatPtr(0.7)},
			machine:   map[string]interface{}{"llm": map[string]interface{}{"temperature": "0.9"}},
			action:    nil,
			guard:     map[string]interface{}{"llm": map[string]interface{}{"temperature": "0.8"}},
			wantTemp:  floatPtr(0.9),
		},
		{
			name:      "machine slices (no hierarchy)",
			appCfg:    &AppConfig{},
			machine:   map[string]interface{}{"llm": map[string]interface{}{"tool_policies": []interface{}{"pol1"}}},
			action:    nil,
			guard:     nil,
			wantSlices: []string{"pol1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(tt.appCfg)
			res := r.Resolve(tt.machine, tt.action, tt.guard)
			assert.Equal(t, tt.wantModel, res.Model)
			if tt.wantTemp != nil {
				assert.Equal(t, *tt.wantTemp, *res.Temperature)
			}
			assert.Equal(t, tt.wantSlices, res.ToolPolicies)
		})
	}
}