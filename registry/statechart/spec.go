// Package statechart provides YAML specs for state machines, parsing to statechartx.Machine.
package statechart

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"github.com/comalice/statechartx"
)

// YamlMachineSpec top-level YAML structure (matches example traffic-light YAML).
type YamlMachineSpec struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description,omitempty"`
	Machine     YamlMachine       `yaml:"machine"`
	LLM         map[string]any    `yaml:"llm,omitempty"`     // For maelstrom config resolver
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

// ToMachine builds statechartx.Machine from spec using public builder.
// Resolves guards/actions as stubs (extend with expr eval, registry, LLM).
func (s *YamlMachineSpec) ToMachine() (*statechartx.Machine, error) {
	if _, ok := s.Machine.States[s.Machine.Initial]; !ok {
		return nil, fmt.Errorf("initial state %q not found", s.Machine.Initial)
	}

	b := statechartx.NewMachineBuilder(s.Machine.ID, s.Machine.Initial)

	if err := s.declareRecursive(b, s.Machine.States, ""); err != nil {
		return nil, fmt.Errorf("declareRecursive: %w", err)
	}
	if err := s.configureRecursive(b, s.Machine.States, ""); err != nil {
		return nil, fmt.Errorf("configureRecursive: %w", err)
	}

	m, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("builder build: %w", err)
	}
	return m, nil
}

// declareRecursive declares states recursively using dot-notation hierarchy (e.g. "parent.child").
func (s *YamlMachineSpec) declareRecursive(b *statechartx.MachineBuilder, states map[string]YamlState, prefix string) error {
	for id, st := range states {
		fullpath := id
		if prefix != "" {
			fullpath = prefix + "." + id
		}
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
		if err := s.declareRecursive(b, st.States, fullpath); err != nil {
			return err
		}
	}
	return nil
}


// configureRecursive configures transitions and timeouts recursively.
func (s *YamlMachineSpec) configureRecursive(b *statechartx.MachineBuilder, states map[string]YamlState, prefix string) error {
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
			log.Printf("[WARN] Timeout %v ignored; add timer logic", st.Timeout)
		}

		for evt, trans := range st.On {
			targetFull := trans.Target
			if prefix != "" && !strings.Contains(trans.Target, ".") && trans.Target != fullpath {
				targetFull = prefix + "." + trans.Target
			}
			guard := s.resolveGuard(trans.Guard)
			action := s.resolveAction(trans.Action)
			sb.On(evt, targetFull, guard, action)
		}
		if err := s.configureRecursive(b, st.States, fullpath); err != nil {
			return err
		}
	}
	return nil
}

// resolveGuard stub: map lookup + expr compiler placeholder.
// Extend: Use goexpr, otto.js, or maelstrom LLM for dynamic eval.
func (s *YamlMachineSpec) resolveGuard(name string) statechartx.Guard {
	if name == "" {
		return nil
	}
	if expr, ok := s.Guards[name]; ok {
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) (bool, error) {
			// TODO: Eval expr vs statechartx.FromContext(ctx), evt.Data
			log.Printf("[Guard %s] expr=%q stub=true", name, expr)
			return true, nil
		}
	}
	return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) (bool, error) {
		log.Printf("[Guard missing %q] default=true", name)
		return true, nil
	}
}

// resolveAction similar stub.
func (s *YamlMachineSpec) resolveAction(name string) statechartx.Action {
	if name == "" {
		return nil
	}
	if expr, ok := s.Actions[name]; ok {
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
			log.Printf("[Action %s] expr=%q stub=executed", name, expr)
			return nil
		}
	}
	return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
		log.Printf("[Action missing %q] noop", name)
		return nil
	}
}
