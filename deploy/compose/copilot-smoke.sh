#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -p gantry-copilot-smoke -f docker-compose.yml)
api_url="${GANTRY_COPILOT_SMOKE_API_URL:-http://localhost:8080}"
keycloak_url="${GANTRY_KEYCLOAK_SMOKE_URL:-http://localhost:8180}"
response_body=""
response_status=""
access_token=""

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    "${compose[@]}" logs keycloak control-plane runner || true
  fi
  "${compose[@]}" down --volumes --remove-orphans || true
  exit "$status"
}
trap cleanup EXIT

json_value() { sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p" <<<"$response_body" | head -n1; }
task_run_id() { sed -n 's/.*"current_run":{"id":"\([^"]*\)".*/\1/p' <<<"$response_body"; }

token_for() {
  local username=$1 password=$2 body
  body=$(curl -sS -X POST "${keycloak_url}/realms/gantry-dev/protocol/openid-connect/token" \
    --data-urlencode grant_type=password \
    --data-urlencode client_id=gantry-copilot-smoke \
    --data-urlencode client_secret=gantry-smoke-secret \
    --data-urlencode "username=${username}" \
    --data-urlencode "password=${password}")
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p' <<<"$body"
}

request() {
  local method=$1 path=$2 payload=${3:-} output
  output=$(mktemp)
  local args=(-sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer ${access_token}")
  if [[ -n $payload ]]; then args+=(-H 'Content-Type: application/json' --data "$payload"); fi
  response_status=$(curl "${args[@]}" "${api_url}${path}")
  response_body=$(<"$output")
  rm -f "$output"
}

wait_task_status() {
  local task_id=$1 expected=$2
  for _ in $(seq 1 80); do
    request GET "/api/copilot/v1/tasks/${task_id}"
    if [[ $response_status == 200 && $(json_value status) == "$expected" ]]; then return 0; fi
    sleep 0.25
  done
  echo "task ${task_id} did not reach ${expected}: ${response_status} ${response_body}" >&2
  return 1
}

submit() {
  local key=$1 payload=$2 output
  output=$(mktemp)
  response_status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/copilot/v1/tasks" \
    -H "Authorization: Bearer ${access_token}" -H "Content-Type: application/json" -H "Idempotency-Key: ${key}" --data "$payload")
  response_body=$(<"$output")
  rm -f "$output"
}

"${compose[@]}" up --build --detach

for _ in $(seq 1 80); do
  access_token=$(token_for copilot-demo gantry_demo_password || true)
  [[ -n $access_token ]] && break
  sleep 0.25
done
[[ -n $access_token ]]

request GET /api/copilot/v1/agents
[[ $response_status == 200 ]]
agent_id=$(json_value id)
[[ $agent_id == agt_lifecycle_demo ]]

complete_key="complete-${RANDOM}-${RANDOM}"
submit "$complete_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"complete\"}"
[[ $response_status == 201 ]]
complete_task=$(json_value id)
[[ -n $complete_task ]]
wait_task_status "$complete_task" completed
submit "$complete_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"complete\"}"
[[ $response_status == 200 && $(json_value id) == "$complete_task" ]]

other_token=$(token_for copilot-other gantry_other_password)
access_token=$other_token
request GET "/api/copilot/v1/tasks/${complete_task}"
[[ $response_status == 404 ]]
access_token=$(token_for copilot-demo gantry_demo_password)

cancel_key="cancel-${RANDOM}-${RANDOM}"
submit "$cancel_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"wait\",\"structured_input\":{\"mode\":\"await_cancel\"}}"
[[ $response_status == 201 ]]
cancel_task=$(json_value id)
cancel_run=$(task_run_id)
wait_task_status "$cancel_task" running
request POST "/api/copilot/v1/tasks/${cancel_task}/runs/${cancel_run}:cancel" '{}'
[[ $response_status == 200 ]]
wait_task_status "$cancel_task" canceled

loss_key="loss-${RANDOM}-${RANDOM}"
submit "$loss_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"loss\",\"structured_input\":{\"mode\":\"await_cancel\"}}"
loss_task=$(json_value id)
wait_task_status "$loss_task" running
"${compose[@]}" kill runner
wait_task_status "$loss_task" failed
"${compose[@]}" up --detach runner

restart_key="restart-${RANDOM}-${RANDOM}"
submit "$restart_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"restart\",\"structured_input\":{\"mode\":\"await_cancel\"}}"
restart_task=$(json_value id)
wait_task_status "$restart_task" running
"${compose[@]}" restart control-plane
wait_task_status "$restart_task" failed
request POST "/api/copilot/v1/tasks/${restart_task}:retry" '{}'
[[ $response_status == 201 ]]
wait_task_status "$restart_task" completed

echo "Copilot persistent lifecycle smoke test passed."
