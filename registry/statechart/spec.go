// Package statechart provides YAML specs for state machines, parsing to statechartx.Machine.
package statechart

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"strings"
	"time"

	"github.com/comalice/maelstrom/internal/llm"
	"github.com/expr-lang/expr"
	"gopkg.in/yaml.v3"
	"github.com/comalice/statechartx"
)

// YamlMachineSpec top-level YAML structure (matches example traffic-light YAML).
type YamlMachineSpec struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description,omitempty"`
	Machine     YamlMachine       `yaml:"machine"`
	LLM         llm.LLMConfig `yaml:"llm,omitempty"`
	Actions     map[string]string `yaml:"actions,omitempty"` // name -> expr/code/ref
	Guards      map[string]string `yaml:"guards,omitempty"`  // name -> expr/code/ref
}

// YamlMachine root.
type YamlMachine struct {
	ID      string            `yaml:"id"`
	Initial string            `yaml:"initial"`
	States  map[string]YamlState `yaml:"states"`
}

// YamlState recursive for hierarchy/compound/parallel.
type YamlState struct {
	Description string                   `yaml:"description,omitempty"`
	Initial     string                   `yaml:"initial,omitempty"`
	Timeout     string                   `yaml:"timeout,omitempty"` // e.g. "30s" -> timer event
	IsParallel  bool                     `yaml:"parallel,omitempty"`
	On          map[string]YamlTransition `yaml:"on,omitempty"`
	States      map[string]YamlState      `yaml:"states,omitempty"` // Compound/children
}

// YamlTransition event config.
type YamlTransition struct {
	Target string `yaml:"target"`
	Guard  string `yaml:"guard,omitempty"`
	Action string `yaml:"action,omitempty"`
}

// ParseSpec unmarshals YAML bytes to spec.
func ParseSpec(data []byte) (*YamlMachineSpec, error) {
	var spec YamlMachineSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return &spec, nil
}

type AugmentedMachine struct {
	Machine        *statechartx.Machine
	StatePathByID  map[statechartx.StateID]string
	StateIDByPath  map[string]statechartx.StateID
	EventIDByName  map[string]statechartx.EventID
	EventNameByID  map[statechartx.EventID]string
}

// ToAugmentedMachine builds statechartx.Machine from spec and adds ID/name mappings.
// Resolves guards/actions as stubs (extend with expr eval, registry, LLM).
func (s *YamlMachineSpec) ToAugmentedMachine() (*AugmentedMachine, error) {
	if _, ok := s.Machine.States[s.Machine.Initial]; !ok {
		return nil, fmt.Errorf("initial state %q not found", s.Machine.Initial)
	}

	initialFullpath := s.Machine.ID + "." + s.Machine.Initial
	b := statechartx.NewMachineBuilder(s.Machine.ID, initialFullpath)
	b.State(s.Machine.ID).Compound(initialFullpath)

	statesSeen := make(map[string]struct{})
	statesSeen[s.Machine.ID] = struct{}{}
	eventsSeen := make(map[string]struct{})

	if err := s.declareRecursive(b, s.Machine.States, s.Machine.ID, &statesSeen); err != nil {
		return nil, fmt.Errorf("declareRecursive: %w", err)
	}
	statesSeen[initialFullpath] = struct{}{}
	if err := s.configureRecursive(b, s.Machine.States, s.Machine.ID, &eventsSeen); err != nil {
		return nil, fmt.Errorf("configureRecursive: %w", err)
	}

	m, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("builder build: %w", err)
	}

	aug := &AugmentedMachine{
		Machine:       m,
		StatePathByID: make(map[statechartx.StateID]string),
		StateIDByPath: make(map[string]statechartx.StateID),
		EventIDByName: make(map[string]statechartx.EventID),
		EventNameByID: make(map[statechartx.EventID]string),
	}
	for path := range statesSeen {
		id := b.GetID(path)
		aug.StateIDByPath[path] = id
		aug.StatePathByID[id] = path
	}
	for evt := range eventsSeen {
		internalKey := "event:" + evt
		eid := b.GetID(internalKey)
		aug.EventIDByName[evt] = statechartx.EventID(eid)
		aug.EventNameByID[statechartx.EventID(eid)] = evt
	}
	return aug, nil
}

func (s *YamlMachineSpec) ToMachine() (*statechartx.Machine, error) {
	aug, err := s.ToAugmentedMachine()
	if err != nil {
		return nil, err
	}
	return aug.Machine, nil
}

// declareRecursive declares states recursively using dot-notation hierarchy (e.g. "parent.child").
func (s *YamlMachineSpec) declareRecursive(b *statechartx.MachineBuilder, states map[string]YamlState, prefix string, statesSeen *map[string]struct{}) error {
	for id, st := range states {
		fullpath := id
		if prefix != "" {
			fullpath = prefix + "." + id
		}
		(*statesSeen)[fullpath] = struct{}{}
		sb := b.State(fullpath)

		if len(st.States) > 0 {
			childInitial := st.Initial
			if childInitial == "" {
				for childID := range st.States {
					childInitial = childID
					break
				}
			}
			sb.Compound(fullpath + "." + childInitial)
		}
		if st.IsParallel {
			sb.Parallel()
		}
		// Recurse children
		if err := s.declareRecursive(b, st.States, fullpath, statesSeen); err != nil {
			return err
		}
	}
	return nil
}


// configureRecursive configures transitions and timeouts recursively.
func (s *YamlMachineSpec) configureRecursive(b *statechartx.MachineBuilder, states map[string]YamlState, prefix string, eventsSeen *map[string]struct{}) error {
	for id, st := range states {
		fullpath := id
		if prefix != "" {
			fullpath = prefix + "." + id
		}
		sb := b.State(fullpath)

		if st.Timeout != "" {
			if _, err := time.ParseDuration(st.Timeout); err != nil {
				return fmt.Errorf("invalid timeout %q: %w", st.Timeout, err)
			}
			slog.Warn("Timeout ignored; add timer logic", "timeout", st.Timeout)
		}

		for evt, trans := range st.On {
			(*eventsSeen)[evt] = struct{}{}
			targetFull := trans.Target
			if !strings.HasPrefix(trans.Target, s.Machine.ID+".") {
				if strings.Contains(trans.Target, ".") {
					targetFull = s.Machine.ID + "." + trans.Target
				} else if prefix != "" && trans.Target != fullpath {
					targetFull = prefix + "." + trans.Target
				}
			}
			guard := s.resolveGuard(trans.Guard)
			action := s.resolveAction(trans.Action)
			sb.On(evt, targetFull, guard, action)
		}
		if err := s.configureRecursive(b, st.States, fullpath, eventsSeen); err != nil {
			return err
		}
	}
	return nil
}

// resolveGuard stub: map lookup + expr compiler placeholder.
// Extend: Use goexpr, otto.js, or maelstrom LLM for dynamic eval.
func getContextData(ctx context.Context) map[string]any {
	if c := statechartx.FromContext(ctx); c != nil {
		return c.GetAll()
	}
	return map[string]any{}
}

func mergeContextData(ctx context.Context, patch map[string]any) {
	if c := statechartx.FromContext(ctx); c != nil {
		c.LoadAll(patch)
	}
}

func (s *YamlMachineSpec) resolveGuard(name string) statechartx.Guard {
	if name == "" {
		return nil
	}
	// Try inline expr first
	prog, err := expr.Compile(name, expr.AsBool())
	if err == nil {
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) (bool, error) {
			ctxData := getContextData(ctx)
			env := map[string]any{
				"ctx": ctxData,
				"evt": evt.Data,
			}
			out, err := expr.Run(prog, env)
			if err != nil {
				slog.Warn("Guard run failed (inline)", "name", name, "env", env, "err", err)
				return true, nil
			}
			if b, ok := out.(bool); ok {
				slog.Info("Guard (inline)", "name", name, "ctx", ctxData, "evt", evt.Data, "result", b)
				return b, nil
			}
			slog.Warn("Guard not bool (inline)", "name", name, "out", out)
			return false, nil
		}
	}
	// Fallback to named guard
	exprStr, ok := s.Guards[name]
	if !ok {
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) (bool, error) {
			slog.Info("Guard missing", "name", name, "default", true)
			return true, nil
		}
	}
	prog, err = expr.Compile(exprStr, expr.AsBool())
	if err != nil {
		slog.Warn("Guard compile failed", "name", name, "expr", exprStr, "err", err)
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) (bool, error) {
			return true, nil
		}
	}
	return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) (bool, error) {
		ctxData := getContextData(ctx)
		env := map[string]any{
			"ctx": ctxData,
			"evt": evt.Data,
		}
		out, err := expr.Run(prog, env)
		if err != nil {
			slog.Warn("Guard run failed", "name", name, "env", env, "err", err)
			return true, nil
		}
		if b, ok := out.(bool); ok {
			slog.Info("Guard", "name", name, "expr", exprStr, "ctx", ctxData, "evt", evt.Data, "result", b)
			return b, nil
		}
		slog.Warn("Guard not bool", "name", name, "out", out)
		return false, nil
	}
}

// resolveAction similar stub.
func (s *YamlMachineSpec) resolveAction(name string) statechartx.Action {
	if name == "" {
		return nil
	}
	actionStr, ok := s.Actions[name]
	if !ok {
		actionStr = name // fallback to inline prompt
	}
	if s.LLM.Provider == "" {
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
			slog.Info("Action no LLM noop", "name", name)
			return nil
		}
	}
	return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
		ctxData := getContextData(ctx)
		evtDataBytes, _ := json.Marshal(evt.Data)
		prompt := fmt.Sprintf(`Action '%s' from %v → %v on event %s with context %v.

%s

Reply ONLY with valid JSON object to shallow merge into context. No other text. Example: {"count": 5}`, name, from, to, string(evtDataBytes), ctxData, actionStr)
		resp, err := llm.Call(ctx, s.LLM, prompt)
		if err != nil {
			slog.Error("Action LLM call failed", "name", name, "err", err)
			return nil
		}
		var patch map[string]any
		if err := json.Unmarshal([]byte(resp), &patch); err != nil {
			slog.Warn("Action JSON parse failed", "name", name, "resp", resp, "err", err)
			return nil
		}
		mergeContextData(ctx, patch)
		slog.Info("Action merged patch", "name", name, "patch", patch)
		return nil
	}
}
