package statechart

import (
	"context"
	"testing"

	"github.com/comalice/maelstrom/internal/llm"
	"github.com/comalice/statechartx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParseSpec(t *testing.T) {
	yamlStr := `
name: traffic-light
version: 1.0
machine:
  id: root
  initial: green
  states:
    green:
      on:
        timer:
          target: yellow
    yellow:
      on:
        timer:
          target: red
    red:
      on:
        timer:
          target: green
guards:
  always: "true"
actions:
  log: "log transition"
`
	spec, err := ParseSpec([]byte(yamlStr))
	require.NoError(t, err)
	assert.Equal(t, "traffic-light", spec.Name)
	assert.Equal(t, "root", spec.Machine.ID)
	assert.Equal(t, "green", spec.Machine.Initial)
	assert.Len(t, spec.Machine.States, 3)
	assert.Contains(t, spec.Guards, "always")
}

func TestToMachine_SimpleTrafficLight(t *testing.T) {
	yamlStr := `
name: traffic-light
version: 1.0
machine:
  id: root
  initial: green
  states:
    green:
      on:
        timer:
          target: yellow
    yellow:
      on:
        timer:
          target: red
    red:
      on:
        timer:
          target: green
`
	spec, err := ParseSpec([]byte(yamlStr))
	require.NoError(t, err)

	machine, err := spec.ToMachine()
	require.NoError(t, err)
	require.NotNil(t, machine)

// Validate runtime starts (implies valid machine)
	rt := statechartx.NewRuntime(machine, nil)
	bgCtx := context.Background()
	require.NoError(t, rt.Start(bgCtx))
	defer func() {
		if err := rt.Stop(); err != nil {
			t.Logf("rt.Stop: %v", err)
		}
	}()
	assert.True(t, rt.IsInState(machine.Initial))
}

func TestToMachine_Hierarchy(t *testing.T) {
	yamlStr := `
name: app
machine:
  id: root
  initial: off
  states:
    off:
      on:
        power_on:
          target: on.idle
    on:
      states:
        idle:
          on:
            start: {target: working}
        working:
          on:
            finish: {target: idle}
`
	spec, err := ParseSpec([]byte(yamlStr))
	require.NoError(t, err)

	machine, err := spec.ToMachine()
	require.NoError(t, err)

	// Builder handles dot-notation: "on.idle" etc.
	assert.NotNil(t, machine)
}

func TestToMachine_GuardsActions(t *testing.T) {
	yamlStr := `
name: guarded
machine:
  id: root
  initial: start
  states:
    start:
      on:
        next:
          target: end
          guard: always
          action: log
    end: {}
guards:
  always: "true"
actions:
  log: "log"
`
	spec, err := ParseSpec([]byte(yamlStr))
	require.NoError(t, err)

	machine, err := spec.ToMachine()
	require.NoError(t, err)

	// Test runtime with stubs
	rt := statechartx.NewRuntime(machine, nil)
	ctx := context.Background()
	require.NoError(t, rt.Start(ctx))
	defer func() {
		if err := rt.Stop(); err != nil {
			t.Logf("rt.Stop: %v", err)
		}
	}()

	// Event ID from builder (event:timer -> assigned ID)
	// Stubs don't need real exec; just no panic
	assert.True(t, rt.IsInState(machine.Initial))
}

func TestToMachine_Errors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			"missing states",
			`machine: {id: root}`,
			"initial state",
		},
		{
			"invalid timeout",
			`machine:
  id: root
  initial: foo
  states:
    foo:
      timeout: invalid`,
			"invalid timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseSpec([]byte(tt.yaml))
			if tt.wantErr == "yaml unmarshal" {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			_, err = spec.ToMachine()
			assert.Error(t, err)
		})
	}
}

func TestYamlStateRecursiveUnmarshal(t *testing.T) {
	// Verify nested states unmarshal correctly
	yamlStr := `
id: root
initial: parent.child
states:
  parent:
    states:
      child:
        parallel: true
        on:
          evt:
            target: sibling
      sibling: {}
`
	var m YamlMachine
	err := yaml.Unmarshal([]byte(yamlStr), &m)
	require.NoError(t, err)
	assert.True(t, m.States["parent"].States["child"].IsParallel)
}

func TestResolveAction(t *testing.T) {
	tests := []struct {
		name string
		spec *YamlMachineSpec
		specIn any
		wantNil bool
	}{
		{
			name: "nil",
			specIn: nil,
			wantNil: true,
		},
		{
			name: "string_fallback",
			specIn: "simple_string_action",
			wantNil: false, // becomes LLM stub if LLM.Provider==""
		},
		{
			name: "legacy_llm",
			spec: &YamlMachineSpec{LLM: llm.LLMConfig{Provider: "anthropic"}},
			specIn: map[string]any{
				"type": "llm",
				"system": "You are helpful.",
				"prompt": "Respond to user.",
			},
			wantNil: false,
		},
		{
			name: "empty_tools",
			spec: &YamlMachineSpec{LLM: llm.LLMConfig{Provider: "anthropic"}},
			specIn: map[string]any{
				"llm_with_tools": map[string]any{
					"tools": []any{},
					"prompt": "JSON patch please.",
				},
			},
			wantNil: false,
		},
		{
			name: "toolful",
			spec: &YamlMachineSpec{LLM: llm.LLMConfig{Provider: "anthropic"}},
			specIn: map[string]any{
				"llm_with_tools": map[string]any{
					"tools": []any{"policy"},
				},
			},
			wantNil: false,
		},
		{
			name: "invalid_nonstring_nonmap",
			specIn: map[string]any{"invalid": 123},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.spec
			if spec == nil {
				spec = &YamlMachineSpec{}
			}
			action := spec.resolveAction(nil, tt.specIn)
			if tt.wantNil {
				assert.Nil(t, action)
			} else {
				assert.NotNil(t, action)
			}
		})
	}
}
