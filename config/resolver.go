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

func getLLMMap(m map[string]interface{}) map[string]interface{} {
	if llm, ok := m["llm"].(map[string]interface{}); ok {
		return llm
	}
	return map[string]interface{}{}
}

func (r *ConfigHierarchyResolver) getString(machineYAML, actionConfig, guardConfig map[string]interface{}, key, def string) string {
	llms := []map[string]interface{}{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
	for _, llm := range llms {
		if v, ok := llm[key].(string); ok {
			return v
		}
	}
	return def
}

func (r *ConfigHierarchyResolver) getStringPtr(machineYAML, actionConfig, guardConfig map[string]interface{}, key string) *string {
	llms := []map[string]interface{}{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
	for _, llm := range llms {
		if v, ok := llm[key].(string); ok && v != "" {
			s := v
			return &s
		}
	}
	return nil
}

func (r *ConfigHierarchyResolver) getFloatPtr(machineYAML, actionConfig, guardConfig map[string]interface{}, key string) *float64 {
	llms := []map[string]interface{}{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
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

func (r *ConfigHierarchyResolver) getIntPtr(machineYAML, actionConfig, guardConfig map[string]interface{}, key string) *int {
	llms := []map[string]interface{}{getLLMMap(actionConfig), getLLMMap(machineYAML), getLLMMap(guardConfig)}
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

func (r *ConfigHierarchyResolver) getStringSlice(m map[string]interface{}, key string) []string {
	if vs, ok := m[key].([]interface{}); ok {
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
		return os.Getenv(strings.TrimPrefix(raw, "env:"))
	}
	return raw
}

func (r *ConfigHierarchyResolver) Resolve(machineYAML, actionConfig, guardConfig map[string]interface{}) *ResolvedMachineConfig {
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

func ToResolvedMap(c *ResolvedMachineConfig) map[string]interface{} {
	return map[string]interface{}{
		"model":          c.Model,
		"provider":       c.Provider,
		"base_url":       c.BaseURL,
		"api_key":        c.APIKey,
		"temperature":    c.Temperature,
		"max_tokens":     c.MaxTokens,
		"tool_policies":  c.ToolPolicies,
		"allowed_actions": c.AllowedActions,
	}
}
