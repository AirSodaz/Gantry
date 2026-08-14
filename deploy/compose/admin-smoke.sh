#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -p gantry-admin-smoke -f docker-compose.yml)
api_url="${GANTRY_ADMIN_SMOKE_API_URL:-http://localhost:8080}"
keycloak_url="${GANTRY_KEYCLOAK_SMOKE_URL:-http://localhost:8180}"
response_body=""
response_status=""
admin_token=""
copilot_token=""

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

token_for() {
  local client_id=$1 client_secret=$2 username=$3 password=$4 body
  body=$(curl -sS -X POST "${keycloak_url}/realms/gantry-dev/protocol/openid-connect/token" \
    --data-urlencode grant_type=password \
    --data-urlencode "client_id=${client_id}" \
    --data-urlencode "client_secret=${client_secret}" \
    --data-urlencode "username=${username}" \
    --data-urlencode "password=${password}")
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p' <<<"$body"
}

request() {
  local token=$1 method=$2 path=$3 payload=${4:-} output
  output=$(mktemp)
  local args=(-sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer ${token}")
  if [[ -n $payload ]]; then args+=(-H 'Content-Type: application/json' --data "$payload"); fi
  response_status=$(curl "${args[@]}" "${api_url}${path}")
  response_body=$(<"$output")
  rm -f "$output"
}

submit_task() {
  local payload=$1 key=$2 output
  output=$(mktemp)
  response_status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/copilot/v1/tasks" \
    -H "Authorization: Bearer ${copilot_token}" -H 'Content-Type: application/json' -H "Idempotency-Key: ${key}" --data "$payload")
  response_body=$(<"$output")
  rm -f "$output"
}

wait_task_status() {
  local task_id=$1 expected=$2
  for _ in $(seq 1 80); do
    request "$copilot_token" GET "/api/copilot/v1/tasks/${task_id}"
    if [[ $response_status == 200 && $(json_value status) == "$expected" ]]; then return 0; fi
    sleep 0.25
  done
  echo "task ${task_id} did not reach ${expected}: ${response_status} ${response_body}" >&2
  return 1
}

"${compose[@]}" up --build --detach
for _ in $(seq 1 80); do
  admin_token=$(token_for gantry-admin-smoke gantry-admin-smoke-secret admin-demo gantry_admin_password || true)
  copilot_token=$(token_for gantry-copilot-smoke gantry-smoke-secret copilot-demo gantry_demo_password || true)
  [[ -n $admin_token && -n $copilot_token ]] && break
  sleep 0.25
done
[[ -n $admin_token && -n $copilot_token ]]

request "$admin_token" GET /api/admin/v1/workspaces
[[ $response_status == 200 ]]
workspace_id=$(json_value id)
[[ $workspace_id == wsp_development ]]

slug="admin-smoke-${RANDOM}-${RANDOM}"
request "$admin_token" POST /api/admin/v1/agents "{\"workspace_id\":\"${workspace_id}\",\"slug\":\"${slug}\",\"display_name\":\"Admin Smoke\",\"description\":\"Published by the Admin smoke test.\",\"category\":\"Development\"}"
[[ $response_status == 201 ]]
agent_id=$(json_value id)
[[ -n $agent_id ]]

request "$admin_token" GET "/api/admin/v1/agents/${agent_id}/draft"
[[ $response_status == 200 ]]
revision=$(sed -n 's/.*"revision":\([0-9]*\).*/\1/p' <<<"$response_body")
[[ $revision == 1 ]]

request "$admin_token" PUT "/api/admin/v1/agents/${agent_id}/draft" '{"spec":{"kind":"gantry.phase0.demo/v1","mode":"complete"}}'
[[ $response_status == 428 ]]

output=$(mktemp)
response_status=$(curl -sS -o "$output" -w '%{http_code}' -X PUT "${api_url}/api/admin/v1/agents/${agent_id}/draft" -H "Authorization: Bearer ${admin_token}" -H 'Content-Type: application/json' -H 'If-Match: 1' --data '{"spec":{"kind":"gantry.phase0.demo/v1","mode":"complete"}}')
response_body=$(<"$output")
rm -f "$output"
[[ $response_status == 200 ]]
revision=$(sed -n 's/.*"revision":\([0-9]*\).*/\1/p' <<<"$response_body")

output=$(mktemp)
response_status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/admin/v1/agents/${agent_id}:publish" -H "Authorization: Bearer ${admin_token}" -H "If-Match: ${revision}" --data '{}')
response_body=$(<"$output")
rm -f "$output"
[[ $response_status == 201 ]]

request "$copilot_token" GET /api/copilot/v1/agents
[[ $response_status == 200 && $response_body == *"${agent_id}"* ]]
submit_task "{\"agent_id\":\"${agent_id}\",\"message\":\"published by Admin\"}" "admin-published-${RANDOM}-${RANDOM}"
[[ $response_status == 201 ]]
task_id=$(json_value id)
wait_task_status "$task_id" completed

request "$admin_token" POST "/api/admin/v1/agents/${agent_id}:retire" '{}'
[[ $response_status == 204 ]]
request "$copilot_token" GET /api/copilot/v1/agents
[[ $response_status == 200 && $response_body != *"${agent_id}"* ]]

echo "Admin agent lifecycle smoke test passed."
