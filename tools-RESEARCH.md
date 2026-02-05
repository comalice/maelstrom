# Tool-Calling Patterns Research (ARCH.md §4.1-4.3)

## Patterns

### Claude API Tool Use
- **Description**: The LLM is provided with tool definitions as JSON schemas in the messages API. The model outputs a `tool_use` content block specifying the tool name and JSON arguments. The system executes the tool and appends a `tool_result` message. This iterates until the model produces a final textual response without tool calls.
- **Pros**:
  - Native LLM-driven tool selection and parallel calls supported.
  - Flexible for dynamic decisions.
- **Cons**:
  - Dependent on LLM's tool-calling accuracy; potential for incorrect tool choice or arg hallucination.
  - API-specific (though adaptable).

### LangChain Agents (ReAct/Tools)
- **Description**: Implements ReAct (Reason + Act) prompting: LLM generates "Thought", then "Action" (tool name + args), receives "Observation" (tool output), repeats until "Final Answer". Tools are Python/JS functions wrapped with schemas.
- **Pros**:
  - Explicit reasoning trace for transparency and debugging.
  - Framework handles loop, parsing, retries.
- **Cons**:
  - Requires robust prompt engineering.
  - Risk of infinite loops without safeguards (max iterations).

### AutoGPT Architecture
- **Description**: Autonomous agent using GPT for task decomposition into subtasks, iterative planning, tool selection/execution in a loop. Prompts include memory, goals, results; self-critiques and reprioritizes.
- **Pros**:
  - Handles open-ended, multi-step tasks without human intervention.
  - Emergent behaviors from chaining.
- **Cons**:
  - High token/API cost from frequent LLM calls.
  - Unpredictable; error propagation in long chains.

## Recommendation for Maelstrom (Phase 4)
Hybrid **YAML-ReAct**:
- **YAML-defined tools**: Declarative config for tools (name, schema/params, impl pointers) loaded at runtime.
- **Iterative LLM loop**: Feed YAML schemas as Claude-style tools to LLM; execute via ReAct pattern (or native tool_use). Persist state in registry/SCX.

**Why hybrid?**
- YAML: Configurable, versionable, no recompiles (Pros: YAML simplicity + LangChain modularity).
- LLM loop: Adaptive intelligence (Pros: Claude/AutoGPT autonomy).
- Avoids cons: YAML prevents hallucinated tools; max iterations/SCX guards loops.

Implementation sketch:
```
Loop:
1. Prompt LLM with YAML tools + context.
2. Parse tool_use -> exec -> append result.
3. Repeat until final_answer or timeout.
```

## Sources
- Claude API tool use based on Anthropic docs (as of 2025): https://docs.anthropic.com/en/docs/tool-use
- LangChain ReAct tutorial: https://python.langchain.com/docs/modules/agents/agent_types/react
- AutoGPT architecture: https://github.com/Significant-Gravitas/AutoGPT
- 2026 searches ("Claude API tool use 2026", etc.) failed due to API errors; summary uses knowledge cutoff Jan 2025.
