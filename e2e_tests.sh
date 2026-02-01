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
    if curl -fs --max-time 1 "http://localhost:$port/" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

test_curl() {
  local label="$1"
  local jq_expr="$2"
  local response
  response=$(curl -fs "http://localhost:$TEST_PORT/api/v1/yamls" || echo '[]')
  [ $VERBOSE = 1 ] && echo "=== $label ===" && echo "$response" | jq '.'
  if ! echo "$response" | jq -e "$jq_expr" >/dev/null 2>&1; then echo "❌ $label failed: $jq_expr"; return 1; fi
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
  wait_ready "$TEST_PORT" || { echo "Server not ready on $TEST_PORT"; kill $SERVER_PID; false; }

  # Initial list empty
  test_curl "initial (empty)" 'length == 0' &&
  # Create valid YAML file (version from filename)
  echo '{}' > "$TEST_DIR/app-v1.0.yaml" &&
  sleep 2  # Wait for watcher &&
  # Listed, active, correct version from filename
  test_curl "after valid.yaml" '
    length == 1 and
    .[0].active == true and
    .[0].version == "1.0" and
    (.?[0].content | type) == "object"
  ' &&
  kill "$SERVER_PID" &&
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
  test_curl "initial-invalid (empty)" 'length == 0' &&
  # Create invalid YAML
  echo 'not valid yaml' > "$TEST_DIR/invalid.yaml" &&
  sleep 2 &&
  # Listed but empty content (unmarshal/render failed)
  test_curl "after-invalid" 'length == 1 and .[0].version == "unknown" and (.[0].content | length) == 0' &&
  kill "$SERVER_PID" &&
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
  if ! echo "$response" | jq -e "$jq_expr" >/dev/null 2>&1; then echo "❌ $label failed: $jq_expr"; return 1; fi
}

test_curl_raw_local() {
  local label="$1"
  local jq_expr="$2"
  local port="$3"
  local response
  response=$(curl -fs "http://localhost:$port/api/v1/raw-yamls" || echo '[]')
  [ $VERBOSE = 1 ] && echo "=== RAW $label ===" && echo "$response" | jq '.'
  if ! echo "$response" | jq -e "$jq_expr" >/dev/null; then echo "❌ RAW $label failed: $jq_expr"; exit 1; fi
}

test_template() {
  local test_port=8084
  TEST_DIR=$(mktemp -d -t maelstrom_test_template_XXXXXX)
  export REGISTRY_DIR="$TEST_DIR"
  export LISTEN_ADDR=":$test_port"
  export FOO=baz
  if [ $VERBOSE = 1 ]; then
    ./bin/server &
  else
    ./bin/server >/dev/null 2>&1 &
  fi
  SERVER_PID=$!
  sleep 0.5
  wait_ready "$test_port" || false

  # Initial empty
  test_curl_local "initial-template (empty)" 'length == 0' $test_port

  # Create templated YAML
  cat > "$TEST_DIR/templ-v1.0.yaml" << 'EOF'
dir: {{ .Config.RegistryDir }}
foo: {{ .Env.FOO }}
EOF
  sleep 2

  # Raw shows template syntax
  test_curl_raw_local "raw after templ" 'length == 1 and (.[0].raw | contains("{{"))' $test_port

  # Rendered shows values (dir from REGISTRY_DIR, FOO=baz)
  test_curl_local "rendered after templ" '
    length == 1 and
    .[0].version == "1.0" and
    .[0].active == true and
    .[0].content.dir == "'"$TEST_DIR"'" and
    .[0].content.foo == "baz"
  ' $test_port

  kill "$SERVER_PID" || true
  echo "✓ Template rendering test passed"
}

TEST_PORT=8081
trap 'kill ${SERVER_PID:-} 2>/dev/null || true; rm -rf "${TEST_DIR:-}"' EXIT

test_valid && test_invalid && test_change && test_template && echo "All E2E tests passed!"
