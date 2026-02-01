# Statechart YAML Specification

This package parses YAML specs into [`statechartx`](https://github.com/comalice/statechartx) machines.

## Schema

Top-level `YamlMachineSpec`:

```yaml
name: string
version: string
description: string (optional)
machine:
  id: string
  initial: string  # Must exist in states
  states:
    state_id:
      description: string (optional)
      initial: child_state_id (optional, for compound)
      timeout: duration (optional, e.g. \"30s\"; warned, unimplemented)
      parallel: bool (optional)
      on:
        event:
          target: state_id (relative/absolute)
          guard: guard_name (optional)
          action: action_name (optional)
      states:  # Recursive for hierarchy
        child_id: ...
actions:  # Global map[string]expr/code/ref (optional)
  name: \"expression/code\"
guards:   # Global map[string]expr/code/ref (optional)
  name: \"expression/code\"
llm: {}   # Maelstrom config resolver (optional)
```

## Examples

### Simple Traffic Light
```yaml
name: traffic-light
version: 1.0
machine:
  id: root
  initial: green
  states:
    green:
      on:
        timer: { target: yellow }
    yellow:
      on:
        timer: { target: red }
    red:
      on:
        timer: { target: green }
```

### Hierarchy
```yaml
name: app
machine:
  id: root
  initial: off
  states:
    off:
      on:
        power_on: { target: on.idle }
    on:
      states:
        idle:
          on:
            start: { target: working }
        working:
          on:
            finish: { target: idle }
```

### Guards/Actions
```yaml
guards:
  always: \"true\"
actions:
  log: \"log transition\"
machine:
  states:
    start:
      on:
        next:
          target: end
          guard: always
          action: log
```

## Parsing

```go
spec, err := ParseSpec(yamlBytes)
machine, err := spec.ToMachine()  // Builds statechartx.Machine
```

Guards/actions are stubs (log only). Extend `resolveGuard`/`resolveAction` for expr/LLM eval.

State paths use dot-notation (e.g. `on.idle`). Compound states auto-pick first child if no `initial`.

## Testing

`go test ./registry/statechart`
