#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -p gantry-admin-smoke -f docker-compose.yml)
api_url="${GANTRY_ADMIN_SMOKE_API_URL:-http://localhost:8080}"
dex_url="${GANTRY_DEX_SMOKE_URL:-http://localhost:5556/dex}"
response_body=""
response_status=""
admin_token=""
copilot_token=""

cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    "${compose[@]}" logs dex control-plane runner || true
  fi
  "${compose[@]}" down --volumes --remove-orphans || true
  exit "$status"
}
trap cleanup EXIT

json_value() { sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p" <<<"$response_body" | head -n1; }
session_id() { sed -n 's/^{"id":"\([^"]*\)".*/\1/p' <<<"$response_body"; }
session_state() { sed -n 's/.*"state":"\([^"]*\)","conversation_revision":[0-9][0-9]*.*/\1/p' <<<"$response_body"; }
first_item_value() { sed -n "s/^{\"items\":\[{[^}]*\"$1\":\"\([^\"]*\)\".*/\1/p" <<<"$response_body"; }

token_for() {
  local client_id=$1 client_secret=$2 username=$3 password=$4 body
  local audience
  case "${client_id}" in
    gantry-admin-smoke) audience=gantry-admin-api ;;
    gantry-copilot-smoke) audience=gantry-copilot-api ;;
    *) echo "unsupported Dex client: ${client_id}" >&2; return 1 ;;
  esac
  body=$(curl -sS -X POST "${dex_url}/token" \
    --user "${client_id}:${client_secret}" \
    --data-urlencode grant_type=password \
    --data-urlencode "scope=openid profile email audience:server:client_id:${audience}" \
    --data-urlencode "username=${username}@example.test" \
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

submit_session() {
  local payload=$1 key=$2 output
  output=$(mktemp)
  response_status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/copilot/v1/sessions" \
    -H "Authorization: Bearer ${copilot_token}" -H 'Content-Type: application/json' -H "Idempotency-Key: ${key}" --data "$payload")
  response_body=$(<"$output")
  rm -f "$output"
}

wait_latest_run_state() {
  local session_id=$1 expected_state=$2 expected_outcome=$3
  for _ in $(seq 1 80); do
    request "$copilot_token" GET "/api/copilot/v1/sessions/${session_id}/runs"
    local current_run_id current_state current_outcome
    current_run_id=$(first_item_value id)
    current_state=$(first_item_value state)
    current_outcome=$(first_item_value outcome)
    if [[ $response_status == 200 && -n $current_run_id && $current_state == "$expected_state" && $current_outcome == "$expected_outcome" ]]; then
      printf '%s\n' "$current_run_id"
      return 0
    fi
    sleep 0.25
  done
  echo "latest Run for Session ${session_id} did not reach ${expected_state}/${expected_outcome}: ${response_status} ${response_body}" >&2
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

request "$admin_token" PUT "/api/admin/v1/agents/${agent_id}/draft" '{"spec":{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"workspace_root":".","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false},"mode":"complete"}}'
[[ $response_status == 428 ]]

output=$(mktemp)
response_status=$(curl -sS -o "$output" -w '%{http_code}' -X PUT "${api_url}/api/admin/v1/agents/${agent_id}/draft" -H "Authorization: Bearer ${admin_token}" -H 'Content-Type: application/json' -H 'If-Match: 1' --data '{"spec":{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"workspace_root":".","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false},"mode":"complete"}}')
response_body=$(<"$output")
rm -f "$output"
[[ $response_status == 200 ]]
revision=$(sed -n 's/.*"revision":\([0-9]*\).*/\1/p' <<<"$response_body")

output=$(mktemp)
response_status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/admin/v1/agents/${agent_id}:review" -H "Authorization: Bearer ${admin_token}" -H 'Content-Type: application/json' -H "If-Match: ${revision}" --data '{"release_notes":"Admin smoke review."}')
response_body=$(<$output)
rm -f "$output"
[[ $response_status == 201 ]]

request "$admin_token" POST "/api/admin/v1/agents/${agent_id}:review-decision" '{"decision":"approve","reason":"Verified by the Admin smoke test."}'
[[ $response_status == 200 ]]

output=$(mktemp)
response_status=$(curl -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/admin/v1/agents/${agent_id}:publish" -H "Authorization: Bearer ${admin_token}" -H "If-Match: ${revision}" --data '{}')
response_body=$(<"$output")
rm -f "$output"
[[ $response_status == 201 ]]

request "$copilot_token" GET /api/copilot/v1/agents
[[ $response_status == 200 && $response_body != *"${agent_id}"* ]]

# The access-management Admin API is not implemented yet. Seed the explicit
# direct-principal grant needed by this smoke without restoring implicit
# workspace-membership authorization.
grant_id="aag_admin_smoke_${RANDOM}_${RANDOM}"
"${compose[@]}" exec -T postgres psql -U gantry -d gantry -v ON_ERROR_STOP=1 \
  -c "INSERT INTO gantry.agent_access_grants (id, agent_id, subject_type, subject_id, state, created_by_principal_id, updated_by_principal_id) VALUES ('${grant_id}', '${agent_id}', 'principal', 'prn_copilot_development', 'active', 'prn_admin_demo', 'prn_admin_demo')" \
  -c "INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('${grant_id}', 'metadata.read'), ('${grant_id}', 'execute')"

request "$copilot_token" GET /api/copilot/v1/agents
[[ $response_status == 200 && $response_body == *"${agent_id}"* ]]
submit_session "{\"agent_id\":\"${agent_id}\",\"message\":\"published by Admin\"}" "admin-published-${RANDOM}-${RANDOM}"
[[ $response_status == 201 ]]
published_session=$(session_id)
wait_latest_run_state "$published_session" completed succeeded >/dev/null
request "$copilot_token" GET "/api/copilot/v1/sessions/${published_session}"
[[ $response_status == 200 && $(session_state) == active ]]

request "$admin_token" POST "/api/admin/v1/agents/${agent_id}:retire" '{}'
[[ $response_status == 204 ]]
request "$copilot_token" GET /api/copilot/v1/agents
[[ $response_status == 200 && $response_body != *"${agent_id}"* ]]

echo "Admin agent lifecycle smoke test passed."
