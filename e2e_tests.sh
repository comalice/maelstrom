#!/bin/bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"  # Project root

TEST_PORT=8081
trap 'kill ${SERVER_PID:-} 2>/dev/null || true; rm -rf "${TEST_DIR:-}"' EXIT

valid_yaml='version: "1.0"
content: {}'
invalid_yaml='invalid: yaml'

test_valid() {
  TEST_DIR=$(mktemp -d -t maelstrom_test_valid_XXXXXX)
  export REGISTRY_DIR="$TEST_DIR"
  export LISTEN_ADDR=":$TEST_PORT"
  ./bin/server >/dev/null 2>&1 &
  SERVER_PID=$!
  sleep 2  # Wait for startup

  # Initial list empty
  curl -fs "http://localhost:$TEST_PORT/api/v1/yamls" | jq -e 'length == 0' >/dev/null

  # Create valid YAML file
  echo -e "$valid_yaml" > "$TEST_DIR/valid.yaml"
  sleep 2  # Wait for watcher

  # Listed, active, correct version
  curl -fs "http://localhost:$TEST_PORT/api/v1/yamls" | jq -e '
    length == 1 and
    .[0].active == true and
    .[0].version == "1.0" and
    (.?[0].content | type) == "object"
  ' >/dev/null

  kill "$SERVER_PID"
  echo "✓ Valid YAML test passed"
}

test_invalid() {
  TEST_PORT=$((TEST_PORT + 1))
  TEST_DIR=$(mktemp -d -t maelstrom_test_invalid_XXXXXX)
  export REGISTRY_DIR="$TEST_DIR"
  export LISTEN_ADDR=":$TEST_PORT"
  ./bin/server >/dev/null 2>&1 &
  SERVER_PID=$!
  sleep 2

  # Initial empty
  curl -fs "http://localhost:$TEST_PORT/api/v1/yamls" | jq -e 'length == 0' >/dev/null

  # Create invalid YAML
  echo -e "$invalid_yaml" > "$TEST_DIR/invalid.yaml"
  sleep 2

  # Still empty (parse failed, skipped)
  curl -fs "http://localhost:$TEST_PORT/api/v1/yamls" | jq -e 'length == 0' >/dev/null

  kill "$SERVER_PID"
  echo "✓ Invalid YAML test passed"
}

test_valid && test_invalid && echo "All E2E tests passed!"
