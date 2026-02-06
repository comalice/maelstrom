# CLI Chat Demo (cli0)

## Quick Start
1. `cd demos/cli0`
2. `cp .env.sample .env` → Edit `LLM_API_KEY=sk-...` (Anthropic)
3. `./chat.sh`
   - Starts server, loads `chat-agent.yaml` (move to `yaml/` if not auto-detected).
   - Chat: `> Hi!` → LLM responds w/ app vars/history.
   - `> status` → View ctx/history.
   - `quit` to exit.

## Features Shown
- **Phase 1**: Env/app vars in YAML (`{{.App.CompanyName}}`).
- **Phase 2**: Statechart loop (idle → thinking → idle), LLM action → JSON patch.
- **Persistence**: Restart via `killall maelstrom; ./chat.sh` → history resumes.

## Logs
Server logs show: YAML resolve → config hierarchy → LLM calls.

## Next
- Add tools: `llm_with_tools` + `tools: [web_search]`.
- Sub-agents: `hire_agent` action.