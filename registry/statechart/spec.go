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
	"github.com/comalice/maelstrom/internal/tools"
	"github.com/expr-lang/expr"
	"gopkg.in/yaml.v3"
	"github.com/comalice/statechartx"
)

type AgentHirer interface {
	HireAgent(template string) error
	RetireAgent(id string) error
	SendMessage(toID string, msg map[string]any) error
	QueryAgents() map[string]AgentInfo
}

type AgentInfo struct {
	ID string `json:"id"`
	Current string `json:"current"`
	History []statechartx.Event `json:"history"`
}






// YamlMachineSpec top-level YAML structure (matches example traffic-light YAML).
type YamlMachineSpec struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description,omitempty"`
	Machine     YamlMachine       `yaml:"machine"`
	LLM         llm.LLMConfig `yaml:"llm,omitempty"`
	Actions     map[string]any `yaml:"actions,omitempty"` // name -> expr/code/ref/map[llm_with_tools]
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
	Action any `yaml:"action,omitempty"`
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
	Spec           *YamlMachineSpec
	Machine        *statechartx.Machine
	StatePathByID  map[statechartx.StateID]string
	StateIDByPath  map[string]statechartx.StateID
	EventIDByName  map[string]statechartx.EventID
	EventNameByID  map[statechartx.EventID]string
}

func (a *AugmentedMachine) Current() string {
	return "idle"
}

func (a *AugmentedMachine) History() []statechartx.Event {
	return []statechartx.Event{}
}

// ToAugmentedMachine builds statechartx.Machine from spec and adds ID/name mappings.
// Resolves guards/actions as stubs (extend with expr eval, registry, LLM).
func (s *YamlMachineSpec) ToAugmentedMachine(hirer AgentHirer) (*AugmentedMachine, error) {
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
	if err := s.configureRecursive(b, s.Machine.States, s.Machine.ID, &eventsSeen, hirer); err != nil {
		return nil, fmt.Errorf("configureRecursive: %w", err)
	}

	m, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("builder build: %w", err)
	}

	aug := &AugmentedMachine{
		Spec:          s,
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
	aug, err := s.ToAugmentedMachine(nil)
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
func (s *YamlMachineSpec) configureRecursive(b *statechartx.MachineBuilder, states map[string]YamlState, prefix string, eventsSeen *map[string]struct{}, hirer AgentHirer) error {
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
			action := s.resolveAction(hirer, trans.Action)
			sb.On(evt, targetFull, guard, action)
		}
		if err := s.configureRecursive(b, st.States, fullpath, eventsSeen, hirer); err != nil {
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

func getString(cfg map[string]any, key string) string {
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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
func (s *YamlMachineSpec) resolveAction(hirer AgentHirer, actionSpec any) statechartx.Action {
	if actionSpec == nil {
		return nil
	}
	var name string
	content := actionSpec
	if strSpec, ok := actionSpec.(string); ok {
		name = strSpec
		if act, ok := s.Actions[name]; ok {
			content = act
		}
	}
// System actions dispatch, e.g. hire_agent:simple
	template, ok := strings.CutPrefix(name, "hire_agent:")
	if ok {
		if hirer == nil {
			return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
				slog.Info("hire_agent system action stub", "template", template)
				return nil
			}
		}
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
			if err := hirer.HireAgent(template); err != nil {
				slog.Error("hire_agent failed", "template", template, "err", err)
				return err
			}
			slog.Info("hired agent via system action", "template", template)
			return nil
		}
	}
	// System actions dispatch, e.g. retire_agent:id123
	id, ok := strings.CutPrefix(name, "retire_agent:")
	if ok {
		if hirer == nil {
			return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
				slog.Info("retire_agent system action stub", "id", id)
				return nil
			}
		}
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
			if err := hirer.RetireAgent(id); err != nil {
				slog.Error("retire_agent failed", "id", id, "err", err)
				return err
			}
			slog.Info("retired agent via system action", "id", id)
			return nil
		}
	}
	// llm_with_tools dispatch
	if toolActionMap, ok := content.(map[string]any); ok {
		// Legacy support for {type: "llm"}
		if typeI, hasType := toolActionMap["type"]; hasType && typeI == "llm" {
			lwt := map[string]any{"tools": []any{}}
			sys := getString(toolActionMap, "system")
			if sys != "" {
				lwt["system"] = sys
			}
			prm := getString(toolActionMap, "prompt")
			if prm != "" {
				lwt["prompt"] = prm
			}
			toolActionMap["llm_with_tools"] = lwt
		}
		lwtI, has := toolActionMap["llm_with_tools"]
		if has {
			lwtCfgI, _ := lwtI.(map[string]any)
			lwtCfg := lwtCfgI
			return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
				ctxData := getContextData(ctx)
				jsonCtxB, _ := json.Marshal(ctxData)
				jsonEvtB, _ := json.Marshal(evt.Data)
				jsonCtx := string(jsonCtxB)
				jsonEvt := string(jsonEvtB)

				system := getString(lwtCfg, "system")
				promptTmpl := getString(lwtCfg, "prompt")
				userPrompt := fmt.Sprintf("%s\n\nCurrent context: %s\nEvent data: %s", promptTmpl, jsonCtx, jsonEvt)

				var maxIter = 5
				if miI, ok := lwtCfg["max_iter"]; ok {
					if mi, ok := miI.(float64); ok {
						maxIter = int(mi)
					} else if mii, ok := miI.(int); ok {
						maxIter = mii
					}
				}

				var toolNames []string
				var hasTools bool
				if toolsArr, ok := lwtCfg["tools"].([]any); ok {
					for _, ti := range toolsArr {
						if ts, ok := ti.(string); ok {
							toolNames = append(toolNames, ts)
						}
					}
				}

				var toolSchemas []tools.ToolSchema
				for _, tn := range toolNames {
					if tool := tools.GlobalTools.Get(tn); tool != nil {
						toolSchemas = append(toolSchemas, tool.Schema())
					} else {
						slog.Warn("tool not found", "name", tn)
					}
				}
				hasTools = len(toolSchemas) > 0

				var systemPrompt string
				if hasTools {
					toolsJSONB, _ := json.MarshalIndent(toolSchemas, "", "  ")
					toolsJSON := string(toolsJSONB)
					systemPrompt = fmt.Sprintf("You have access to these tools. To use a tool, output ONLY {\"tool_use\": {\"name\": \"tool_name\", \"params\": {...}}}\n\n%s\n\nTool results provided next message.\n\nWhen finished, output JSON patch for context (no tool_use).", toolsJSON)
				} else {
					systemPrompt = `You are a helpful assistant.
Reply ONLY with valid JSON: {"response": "your reply here"}. No other text or keys.`
				}
				if system != "" {
					systemPrompt += "\n\n" + system
				}
				if !hasTools {
					maxIter = 1
				}

				msgs := []string{systemPrompt, userPrompt}
				for iter := 0; iter < maxIter; iter++ {
					fullPrompt := strings.Join(msgs, "\n\n\n---\n\n")
					resp, err := llm.DefaultCaller.Call(ctx, s.LLM, fullPrompt)
					if err != nil {
						slog.Error("llm_with_tools LLM call failed", "iter", iter, "err", err)
						return err
					}

					var respMap map[string]any
					if err := json.Unmarshal([]byte(resp), &respMap); err != nil {
						trunc := resp
						if len(resp) > 300 {
							trunc = resp[:300] + "..."
						}
						slog.Warn("llm_with_tools non-JSON, try patch", "resp", trunc, "err", err)
						var patch map[string]any
						if jerr := json.Unmarshal([]byte(resp), &patch); jerr == nil {
							mergeContextData(ctx, patch)
						}
						return nil
					}

					if toolUseI, hasTU := respMap["tool_use"]; hasTU && toolUseI != nil {
						if tuMap, ok := toolUseI.(map[string]any); ok && tuMap != nil {
							if tnameI, hasName := tuMap["name"]; hasName && tnameI != nil {
								if tname, tnOK := tnameI.(string); tnOK {
									if tparamsI, hasParams := tuMap["params"]; hasParams && tparamsI != nil {
										if tparams, tpOK := tparamsI.(map[string]any); tpOK && tparams != nil {
											if tool := tools.GlobalTools.Get(tname); tool != nil {
												toolRes, terr := tool.Execute(ctx, tparams)
												if terr != nil {
													msgs = append(msgs, fmt.Sprintf("Tool '%s' failed: %v", tname, terr))
												} else {
													res := tools.Result{Content: toolRes}
													resJSONB, _ := json.MarshalIndent(res, "", "  ")
													msgs = append(msgs, fmt.Sprintf("Tool '%s' result:\n%s", tname, string(resJSONB)))
												}
												continue
											}
										}
									}
								}
							}
						} else {
							slog.Warn("tool_use not map object", "tool_use", toolUseI)
						}
					}
					// final
					mergeContextData(ctx, respMap)
					slog.Info("llm_with_tools completed", "final_patch", respMap)
					return nil
				}
				slog.Warn("llm_with_tools max iterations reached without final")
				return nil
			}
		}
	}

	// fallback simple LLM action
	actionStr, isStr := content.(string)
	if !isStr {
		slog.Warn("non-string non-llm_with_tools action skipped", "name", name, "content_type", fmt.Sprintf("%T", content))
		return nil
	}
	if s.LLM.Provider == "" {
		return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
			slog.Info("Action no LLM noop", "name", name)
			return nil
		}
	}
	return func(ctx context.Context, evt *statechartx.Event, from, to statechartx.StateID) error {
		ctxData := getContextData(ctx)
		jsonCtxB, _ := json.Marshal(ctxData)
		jsonEvtB, _ := json.Marshal(evt.Data)
		prompt := fmt.Sprintf(`Action '%s'.
State transition from %d to %d.
Current context: %s
Event data: %s

%s

Reply ONLY with valid JSON object to merge into context. No other text.
Example: {"key": "value", "count": 5}`, name, from, to, string(jsonCtxB), string(jsonEvtB), actionStr)
		resp, err := llm.DefaultCaller.Call(ctx, s.LLM, prompt)
		if err != nil {
			slog.Error("Action LLM call failed", "name", name, "err", err)
			return nil
		}
		var patch map[string]any
		if err := json.Unmarshal([]byte(resp), &patch); err != nil {
			trunc := resp
			if len(resp) > 300 {
				trunc = resp[:300] + "..."
			}
			slog.Warn("Action JSON parse failed", "name", name, "resp", trunc, "err", err)
			return nil
		}
		mergeContextData(ctx, patch)
		slog.Info("Action simple LLM merged patch", "name", name, "patch", patch)
		return nil
	}
}
