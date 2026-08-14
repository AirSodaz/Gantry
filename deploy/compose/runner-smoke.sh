#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -p gantry-runner-smoke -f docker-compose.yml)
api_url="${GANTRY_RUNNER_SMOKE_API_URL:-http://localhost:8080}"
export GANTRY_DEVELOPMENT_API_TOKEN="${GANTRY_DEVELOPMENT_API_TOKEN:-gantry_development_token}"
response_body=""
response_status=""

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    "${compose[@]}" logs control-plane runner || true
  fi
  "${compose[@]}" down --volumes --remove-orphans || true
  exit "$status"
}
trap cleanup EXIT

request() {
  local method=$1
  local path=$2
  local payload=${3:-}
  local output
  output=$(mktemp)
  local args=(-sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer ${GANTRY_DEVELOPMENT_API_TOKEN}")
  if [[ -n $payload ]]; then
    args+=(-H 'Content-Type: application/json' --data "$payload")
  fi
  response_status=$(curl "${args[@]}" "${api_url}${path}")
  response_body=$(<"$output")
  rm -f "$output"
}

json_value() {
  local key=$1
  sed -n "s/.*\"${key}\":\"\([^\"]*\)\".*/\1/p" <<<"$response_body"
}

wait_for_status() {
  local run_id=$1
  local expected=$2
  for _ in $(seq 1 60); do
    request GET "/internal/development/runs/${run_id}"
    if [[ $response_status == 200 && $(json_value status) == "$expected" ]]; then
      return 0
    fi
    sleep 0.25
  done
  echo "run ${run_id} did not reach ${expected}: HTTP ${response_status} ${response_body}" >&2
  return 1
}

start_run() {
  local mode=$1
  for _ in $(seq 1 60); do
    request POST /internal/development/runs "{\"mode\":\"${mode}\"}"
    if [[ $response_status == 201 ]]; then
      json_value run_id
      return 0
    fi
    if [[ $response_status != 409 ]]; then
      echo "could not create ${mode} run: HTTP ${response_status} ${response_body}" >&2
      return 1
    fi
    sleep 0.25
  done
  echo "runner did not become available" >&2
  return 1
}

"${compose[@]}" up --build --detach

complete_run=$(start_run complete)
wait_for_status "$complete_run" completed

cancel_run=$(start_run await_cancel)
wait_for_status "$cancel_run" running
request POST "/internal/development/runs/${cancel_run}/cancel"
[[ $response_status == 202 ]]
wait_for_status "$cancel_run" canceled

lost_runner_run=$(start_run await_cancel)
wait_for_status "$lost_runner_run" running
"${compose[@]}" kill runner
wait_for_status "$lost_runner_run" failed

echo "Runner Compose lifecycle smoke test passed."
