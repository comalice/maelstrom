#!/bin/bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"  # Project root

VERBOSE=0
while getopts "v" opt; do
  case $opt in
    v) VERBOSE=1 ;;
  esac
done

wait_ready() {
  local port="$1"
  for i in {1..30}; do
    if curl -fs --max-time 1 "http://localhost:$port/" >/dev/null 2>/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

TEST_PORT=8081
trap 'kill ${SERVER_PID:-} 2>/dev/null || true; rm -rf "${TEST_DIR:-}"' EXIT

test_curl() {
  local label="$1"
  local jq_expr="$2"
  local response
  response=$(curl -fs "http://localhost:$TEST_PORT/api/v1/yamls" || echo '[]')
  [ $VERBOSE = 1 ] && echo "=== $label ===" && echo "$response" | jq '.'
  echo "$response" | jq -e "$jq_expr" >/dev/null || { echo "❌ $label failed: $jq_expr"; false; }
}

test_valid() {
  TEST_DIR=$(mktemp -d -t maelstrom_test_valid_XXXXXX)
  export REGISTRY_DIR="$TEST_DIR"
  export LISTEN_ADDR=":$TEST_PORT"
  if [ $VERBOSE = 1 ]; then
    ./bin/server &
  else
    ./bin/server >/dev/null 2>&1 &
  fi
  SERVER_PID=$!
  sleep 0.5
  wait_ready "$TEST_PORT" || { echo "Server not ready on $TEST_PORT"; cat /proc/$SERVER_PID/cmdline || true; kill $SERVER_PID; false; }

  # Initial list empty
  test_curl "initial (empty)" 'length == 0'

  # Create valid YAML file (version from filename)
  echo '{}' > "$TEST_DIR/app-v1.0.yaml"
  sleep 2  # Wait for watcher

  # Listed, active, correct version from filename
  test_curl "after valid.yaml" '
    length == 1 and
    .[0].active == true and
    .[0].version == "1.0" and
    (.?[0].content | type) == "object"
  '

  kill "$SERVER_PID"
  echo "✓ Valid YAML test passed"
}

test_invalid() {
  TEST_PORT=$((TEST_PORT + 1))
  TEST_DIR=$(mktemp -d -t maelstrom_test_invalid_XXXXXX)
  export REGISTRY_DIR="$TEST_DIR"
  export LISTEN_ADDR=":$TEST_PORT"
  if [ $VERBOSE = 1 ]; then
    ./bin/server &
  else
    ./bin/server >/dev/null 2>&1 &
  fi
  SERVER_PID=$!
  sleep 0.5
  wait_ready "$TEST_PORT" || { echo "Server not ready on $TEST_PORT"; false; }

  # Initial empty
  test_curl "initial-invalid (empty)" 'length == 0'

  # Create invalid YAML
  echo 'not valid yaml' > "$TEST_DIR/invalid.yaml"
  sleep 2

  # Still empty (parse failed, skipped)
  test_curl "after-invalid (empty)" 'length == 0'

  kill "$SERVER_PID"
  echo "✓ Invalid YAML test passed"
}

test_change() {
  local test_port=8083
  TEST_DIR=$(mktemp -d -t maelstrom_test_change_XXXXXX)
  export REGISTRY_DIR="$TEST_DIR"
  export LISTEN_ADDR=":$test_port"
  if [ $VERBOSE = 1 ]; then
    ./bin/server &
  else
    ./bin/server >/dev/null 2>&1 &
  fi
  SERVER_PID=$!
  sleep 0.5
  wait_ready "$test_port" || false

  # Create v1.0
  echo '{}' > "$TEST_DIR/app-v1.0.yaml"
  sleep 2
  test_curl_local "after-v1.0" '
    length == 1 and
    .[0].version == "1.0" and
    .[0].active == true
  ' $test_port

  # Remove v1.0 (deactivate)
  rm "$TEST_DIR/app-v1.0.yaml"
  sleep 1
  test_curl_local "after-rm-v1" '
    length == 1 and
    .[0].version == "1.0" and
    .[0].active == false
  ' $test_port

  # Create v2.0 (new active version)
  echo '{}' > "$TEST_DIR/app-v2.0.yaml"
  sleep 2
  test_curl_local "after-v2.0" '
    length == 2 and
    (map(select(.active)) | length) == 1 and
    (map(select(.active))[0].version == "2.0") and
    (map(select(.active))[0].active == true)
  ' $test_port

  kill "$SERVER_PID" || true
  echo "✓ Change/version bump test passed"
}

test_curl_local() {
  local label="$1"
  local jq_expr="$2"
  local port="$3"
  local response
  response=$(curl -fs "http://localhost:$port/api/v1/yamls" || echo '[]')
  [ $VERBOSE = 1 ] && echo "=== $label ===" && echo "$response" | jq '.'
  echo "$response" | jq -e "$jq_expr" >/dev/null || { echo "❌ $label failed: $jq_expr"; false; }
}

test_valid && test_invalid && test_change && echo "All E2E tests passed!"
