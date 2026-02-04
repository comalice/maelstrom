package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/comalice/maelstrom/config"
	"github.com/comalice/maelstrom/internal/llm"
	"github.com/comalice/maelstrom/registry"
	"github.com/comalice/statechartx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestE2E_LLMHierarchy(t *testing.T) {
	origCaller := llm.DefaultCaller
	defer func() { llm.DefaultCaller = origCaller }()

	tests := []struct {
		name        string
		appCfg      *config.AppConfig
		yamlContent string
		wantCalls   int
		wantConfig  llm.LLMConfig
	}{
		{
			name: "TestActualLLMCallUsesCorrectProvider",
			appCfg: &config.AppConfig{
				DefaultProvider: "app-prov",
				DefaultModel:    "app-model",
			},
			yamlContent: `name: test
machine:
  id: root
  initial: idle
  states:
    idle:
      on:
        trigger:
          target: done
          action: "test action"
    done: {}`,
			wantCalls: 1,
			wantConfig: llm.LLMConfig{
				Provider: "app-prov",
				Model:    "app-model",
			},
		},
		{
			name: "TestMachineOverride",
			appCfg: &config.AppConfig{
				DefaultProvider: "app-prov",
			},
			yamlContent: `name: test
llm:
  provider: yaml-prov
machine:
  id: root
  initial: idle
  states:
    idle:
      on:
        trigger:
          target: done
          action: "test action"
    done: {}`,
			wantCalls: 1,
			wantConfig: llm.LLMConfig{
				Provider: "yaml-prov",
			},
		},
		{
			name: "TestGranularModel",
			appCfg: &config.AppConfig{
				DefaultModel: "app-model",
				DefaultProvider: "app-prov",
			},
			yamlContent: `name: test
llm:
  model: yaml-model
machine:
  id: root
  initial: idle
  states:
    idle:
      on:
        trigger:
          target: done
          action: "test action"
    done: {}`,
			wantCalls: 1,
			wantConfig: llm.LLMConfig{
				Provider: "app-prov",
				Model:    "yaml-model",
			},
		},
		{
			name:       "TestNoProviderNoop",
			appCfg:     &config.AppConfig{},
			yamlContent: `name: test
machine:
  id: root
  initial: idle
  states:
    idle:
      on:
        trigger:
          target: done
          action: "test action"
    done: {}`,
			wantCalls:  0,
		},
		{
			name: "TestEdgeAPIKeyEnv",
			appCfg: &config.AppConfig{
				DefaultProvider: "app-prov",
				DefaultAPIKey:   "env:TEST_KEY",
			},
			yamlContent: `name: test
machine:
  id: root
  initial: idle
  states:
    idle:
      on:
        trigger:
          target: done
          action: "test action"
    done: {}`,
			wantCalls: 1,
			wantConfig: llm.LLMConfig{
				Provider: "app-prov",
				APIKey:   "secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_KEY", "secret")
			defer os.Unsetenv("TEST_KEY")

			llm.DefaultCaller = &llm.MockCaller{}
			mock := llm.DefaultCaller.(*llm.MockCaller)
			mock.ResetCalls()

			dir := t.TempDir()
			filename := "test.yaml"
			fullpath := filepath.Join(dir, filename)
			require.NoError(t, os.WriteFile(fullpath, []byte(tt.yamlContent), 0644))

			r := registry.New()
			r.SetDir(dir)
			r.SetConfig(tt.appCfg)
			require.NoError(t, r.Import(filename))

			list := r.List()
			require.Len(t, list, 1)
			item := list[0]
			require.NotNil(t, item.StatechartAugmented)

			rt := statechartx.NewRuntime(item.StatechartAugmented.Machine, nil)
			ctx := context.Background()
			require.NoError(t, rt.Start(ctx))
			defer func() { _ = rt.Stop() }()

			eid, ok := item.StatechartAugmented.EventIDByName["trigger"]
			require.True(t, ok)
			rt.ProcessEvent(statechartx.Event{ID: eid})

			assert.Equal(t, tt.wantCalls, len(mock.Calls))
			if tt.wantCalls > 0 {
				assert.Equal(t, tt.wantConfig.Provider, mock.Calls[0].Config.Provider)
				assert.Equal(t, tt.wantConfig.Model, mock.Calls[0].Config.Model)
			}
		})
	}
}

func TestE2E_MultiProviderWorkflow(t *testing.T) {
	origCaller := llm.DefaultCaller
	defer func() { llm.DefaultCaller = origCaller }()

	os.Setenv("TEST_KEY", "secret")
	defer os.Unsetenv("TEST_KEY")

	llm.DefaultCaller = &llm.MockCaller{}
	mock := llm.DefaultCaller.(*llm.MockCaller)

	dir := t.TempDir()
	filename := "test.yaml"
	fullpath := filepath.Join(dir, filename)
	yamlContent := `name: test
machine:
  id: root
  initial: idle
  states:
    idle:
      on:
        trigger:
          target: done
          action: "test action"
    done: {}`

	require.NoError(t, os.WriteFile(fullpath, []byte(yamlContent), 0644))

	r := registry.New()
	r.SetDir(dir)
	require.NoError(t, r.Import(filename))

	// First with provA
	appCfgA := &config.AppConfig{DefaultProvider: "provA"}
	r.SetConfig(appCfgA)
	list := r.List()
	require.Len(t, list, 1)
	item := list[0]
	require.NotNil(t, item.StatechartAugmented)

	var eid statechartx.EventID

	rtA := statechartx.NewRuntime(item.StatechartAugmented.Machine, nil)
	ctx := context.Background()
	require.NoError(t, rtA.Start(ctx))
	eid = item.StatechartAugmented.EventIDByName["trigger"]
	require.Contains(t, item.StatechartAugmented.EventIDByName, "trigger")
	rtA.ProcessEvent(statechartx.Event{ID: eid})
_ = rtA.Stop()
	assert.Equal(t, 1, len(mock.Calls))
	assert.Equal(t, "provA", mock.Calls[0].Config.Provider)
	mock.ResetCalls()

	// Reload with provB
	appCfgB := &config.AppConfig{DefaultProvider: "provB"}
	r.SetConfig(appCfgB)
	list = r.List()
	require.Len(t, list, 1)
	item = list[0]
	require.NotNil(t, item.StatechartAugmented)

	rtB := statechartx.NewRuntime(item.StatechartAugmented.Machine, nil)
	require.NoError(t, rtB.Start(ctx))
	eid = item.StatechartAugmented.EventIDByName["trigger"]
	require.Contains(t, item.StatechartAugmented.EventIDByName, "trigger")
	rtB.ProcessEvent(statechartx.Event{ID: eid})
_ = rtB.Stop()
	assert.Equal(t, 1, len(mock.Calls))
	assert.Equal(t, "provB", mock.Calls[0].Config.Provider)
}