package config

import (
	"os"
	"strconv"
	"strings"
)

type ResolvedMachineConfig struct {
	Model          string
	Provider       string
	BaseURL        *string
	APIKey         string
	Temperature    *float64
	MaxTokens      *int
	ToolPolicies   []string
	AllowedActions []string
}

type ConfigHierarchyResolver struct {
	cfg *AppConfig
}

func NewResolver(cfg *AppConfig) *ConfigHierarchyResolver {
	return &ConfigHierarchyResolver{cfg: cfg}
}

func getLLMMap(m map[string]any) map[string]any {
	if llm, ok := m["llm"].(map[string]any); ok {
		return llm
	}
	return map[string]any{}
}

func (r *ConfigHierarchyResolver) getString(machineYAML, actionConfig, guardConfig map[string]any, key, def string) string {
	llms := []map[string]any{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
	for _, llm := range llms {
		if v, ok := llm[key].(string); ok {
			return v
		}
	}
	return def
}

func (r *ConfigHierarchyResolver) getStringPtr(machineYAML, actionConfig, guardConfig map[string]any, key string) *string {
	llms := []map[string]any{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
	for _, llm := range llms {
		if v, ok := llm[key].(string); ok && v != "" {
			s := v
			return &s
		}
	}
	return nil
}

func (r *ConfigHierarchyResolver) getFloatPtr(machineYAML, actionConfig, guardConfig map[string]any, key string) *float64 {
	llms := []map[string]any{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
	for _, llm := range llms {
		if v, ok := llm[key].(string); ok && v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				ff := f
				return &ff
			}
		} else if v, ok := llm[key].(float64); ok {
			ff := v
			return &ff
		}
	}
	return nil
}

func (r *ConfigHierarchyResolver) getIntPtr(machineYAML, actionConfig, guardConfig map[string]any, key string) *int {
	llms := []map[string]any{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
	for _, llm := range llms {
		if v, ok := llm[key].(string); ok && v != "" {
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				ii := int(i)
				return &ii
			}
		} else if v, ok := llm[key].(float64); ok {
			ii := int(v)
			return &ii
		} else if v, ok := llm[key].(int); ok {
			ii := int(v)
			return &ii
		} else if v, ok := llm[key].(int64); ok {
			ii := int(v)
			return &ii
		}
	}
	return nil
}

func (r *ConfigHierarchyResolver) getStringSlice(m map[string]any, key string) []string {
	if vs, ok := m[key].([]any); ok {
		res := make([]string, 0, len(vs))
		for _, vv := range vs {
			if s, ok := vv.(string); ok {
				res = append(res, s)
			}
		}
		return res
	}
	return nil
}

func (r *ConfigHierarchyResolver) resolveAPIKey(raw string) string {
	if strings.HasPrefix(raw, "env:") {
		key := strings.TrimPrefix(raw, "env:")
		if key != "" {
			return os.Getenv(key)
		}
	}
	return raw
}

func (r *ConfigHierarchyResolver) Resolve(machineYAML, actionConfig, guardConfig map[string]any) *ResolvedMachineConfig {
	baseURL := r.getStringPtr(machineYAML, actionConfig, guardConfig, "base_url")
	if baseURL == nil {
		baseURL = r.cfg.DefaultBaseURL
	}
	temperature := r.getFloatPtr(machineYAML, actionConfig, guardConfig, "temperature")
	if temperature == nil {
		defTemp := r.cfg.DefaultTemperature
		temperature = defTemp
	}
	maxTokens := r.getIntPtr(machineYAML, actionConfig, guardConfig, "max_tokens")
	if maxTokens == nil {
		defMax := r.cfg.DefaultMaxTokens
		maxTokens = defMax
	}

	res := &ResolvedMachineConfig{
		Model:          r.getString(machineYAML, actionConfig, guardConfig, "model", r.cfg.DefaultModel),
		Provider:       r.getString(machineYAML, actionConfig, guardConfig, "provider", r.cfg.DefaultProvider),
		BaseURL:        baseURL,
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		APIKey:         r.resolveAPIKey(r.getString(machineYAML, actionConfig, guardConfig, "api_key", r.cfg.DefaultAPIKey)),
		ToolPolicies:   r.getStringSlice(getLLMMap(machineYAML), "tool_policies"),
		AllowedActions: r.getStringSlice(getLLMMap(machineYAML), "allowed_actions"),
	}
	return res
}

func ToResolvedMap(c *ResolvedMachineConfig) map[string]any {
	m := map[string]any{
		"model":      c.Model,
		"provider":   c.Provider,
		"api_key":    c.APIKey,
	}
	if c.BaseURL != nil {
		m["base_url"] = c.BaseURL
	} else {
		m["base_url"] = (*string)(nil)
	}
	if c.Temperature != nil {
		m["temperature"] = c.Temperature
	} else {
		m["temperature"] = (*float64)(nil)
	}
	if c.MaxTokens != nil {
		m["max_tokens"] = c.MaxTokens
	} else {
		m["max_tokens"] = (*int)(nil)
	}
	if c.ToolPolicies != nil {
		m["tool_policies"] = c.ToolPolicies
	} else {
		m["tool_policies"] = ([]string)(nil)
	}
	if c.AllowedActions != nil {
		m["allowed_actions"] = c.AllowedActions
	} else {
		m["allowed_actions"] = ([]string)(nil)
	}
	return m
}
