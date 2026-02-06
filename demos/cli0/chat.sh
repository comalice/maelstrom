#!/bin/bash
set -e

# Load env vars (source .env if exists)
if [ -f .env ]; then
  set -a  # auto export
  source .env
  set +a
fi

BASE="localhost:8090"
echo "🚀 Starting server in background (watch logs)..."
go run ../../cmd/server/maelstrom.go &
SERVER_PID=$!
sleep 5  # Wait for startup/load

# Verify chat-agent loaded
echo "📋 Checking loaded machines:"
curl -s "$BASE/api/v1/statecharts/" | jq | grep chat-agent || echo "Load yaml/demos/cli0/chat-agent.yaml manually if needed"

# Create instance
ID=$(curl -s -d '{"initialContext":{"history":[]}}' "$BASE/api/v1/statecharts/chat-agent/instances" | jq -r '.id // empty')
if [ -z "$ID" ]; then
  echo "❌ chat-agent not found. Copy yaml/demos/cli0/chat-agent.yaml to yaml/ and restart."
  kill $SERVER_PID
  exit 1
fi
echo "🆔 Agent ready: $ID (type 'quit' to exit)"

# Chat loop
while true; do
  read -p "> " MSG || break
  [[ "$MSG" == "quit" ]] && break
  [[ "$MSG" == "status" ]] && { curl -s "$BASE/api/v1/statecharts/chat-agent/instances/$ID" | jq .context; continue; }

  curl -s -X POST -d "{\"type\":\"MESSAGE\",\"data\":{\"message\":\"$MSG\"}}" \
    "$BASE/api/v1/statecharts/chat-agent/instances/$ID/events" | jq .  # Discard /events response (current/history); real resp in next GET
  RESP=$(curl -s "$BASE/api/v1/statecharts/chat-agent/instances/$ID" | jq -r '(.context.response // .context.message // "No response")')
  echo "🤖 $RESP"
done

# Cleanup
kill $SERVER_PID 2>/dev/null || true
echo "✅ Demo done. History persisted: ls ../../instances/chat-agent/"