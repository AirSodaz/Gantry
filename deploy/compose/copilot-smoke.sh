#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -p gantry-copilot-smoke -f docker-compose.yml)
api_url="${GANTRY_COPILOT_SMOKE_API_URL:-http://localhost:8080}"
dex_url="${GANTRY_DEX_SMOKE_URL:-http://localhost:5556/dex}"
response_body=""
response_status=""
response_etag=""
access_token=""

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
run_id() { sed -n 's/^{"id":"\([^"]*\)".*/\1/p' <<<"$response_body"; }
first_item_value() { sed -n "s/^{\"items\":\[{[^}]*\"$1\":\"\([^\"]*\)\".*/\1/p" <<<"$response_body"; }
first_item_number() { sed -n "s/^{\"items\":\[{[^}]*\"$1\":\([0-9][0-9]*\).*/\1/p" <<<"$response_body"; }

token_for() {
  local username=$1 password=$2 body
  body=$(curl --noproxy '*' -sS -X POST "${dex_url}/token" \
    --user gantry-copilot-smoke:gantry-smoke-secret \
    --data-urlencode grant_type=password \
    --data-urlencode scope='openid profile email audience:server:client_id:gantry-copilot-api' \
    --data-urlencode "username=${username}@example.test" \
    --data-urlencode "password=${password}")
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p' <<<"$body"
}

request() {
  local method=$1 path=$2 payload=${3:-} idempotency_key=${4:-} if_match=${5:-} output headers
  output=$(mktemp)
  headers=$(mktemp)
  local args=(-sS -D "$headers" -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer ${access_token}")
  if [[ -n $payload ]]; then args+=(-H 'Content-Type: application/json' --data "$payload"); fi
  if [[ -n $idempotency_key ]]; then args+=(-H "Idempotency-Key: ${idempotency_key}"); fi
  if [[ -n $if_match ]]; then args+=(-H "If-Match: ${if_match}"); fi
  response_status=$(curl --noproxy '*' "${args[@]}" "${api_url}${path}")
  response_body=$(<"$output")
  response_etag=$(tr -d '\r' <"$headers" | sed -n 's/^[Ee][Tt][Aa][Gg]:[[:space:]]*//p' | tail -n1)
  rm -f "$output" "$headers"
}

wait_latest_run_state() {
  local session_id=$1 expected_state=$2 expected_outcome=$3 expected_run_id=${4:-}
  for _ in $(seq 1 80); do
    request GET "/api/copilot/v1/sessions/${session_id}/runs"
    local current_run_id current_state current_outcome
    current_run_id=$(first_item_value id)
    current_state=$(first_item_value state)
    current_outcome=$(first_item_value outcome)
    if [[ $response_status == 200 && -n $current_run_id && $current_state == "$expected_state" && $current_outcome == "$expected_outcome" && ( -z $expected_run_id || $current_run_id == "$expected_run_id" ) ]]; then
      printf '%s\n' "$current_run_id"
      return 0
    fi
    sleep 0.25
  done
  echo "latest Run for Session ${session_id} did not reach ${expected_state}/${expected_outcome}: ${response_status} ${response_body}" >&2
  return 1
}

assert_session_active() {
  local session_id=$1
  request GET "/api/copilot/v1/sessions/${session_id}"
  [[ $response_status == 200 && $(session_state) == active ]]
}

submit_session() {
  local key=$1 payload=$2 output
  output=$(mktemp)
  response_status=$(curl --noproxy '*' -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/copilot/v1/sessions" \
    -H "Authorization: Bearer ${access_token}" -H "Content-Type: application/json" -H "Idempotency-Key: ${key}" --data "$payload")
  response_body=$(<"$output")
  rm -f "$output"
}

assert_action_projection() {
  local run_id=$1 expected=$2 projection
  projection=$("${compose[@]}" exec -T postgres psql -U gantry -d gantry -Atqc "SELECT a.state || '|' || (a.runner_call_id <> '')::text || '|' || COALESCE(a.arguments_json->>'command','') || '|' || (SELECT count(*) FROM gantry.run_events e WHERE e.run_id=a.run_id AND e.event_type IN ('tool.call.completed','tool.call.failed')) || '|' || (a.execution_claimed_at IS NOT NULL)::text FROM gantry.actions a WHERE a.run_id='${run_id}'")
  [[ $projection == "$expected" ]] || { echo "unexpected action projection for ${run_id}: ${projection}" >&2; return 1; }
}

wait_artifact() {
  local session_id=$1
  for _ in $(seq 1 80); do
    local value
    value=$("${compose[@]}" exec -T postgres psql -U gantry -d gantry -Atqc "SELECT id FROM gantry.artifacts WHERE session_id='${session_id}' AND state='available' AND scan_status='passed' LIMIT 1")
    [[ -n $value ]] && { printf '%s\n' "$value"; return 0; }
    sleep 0.25
  done
  echo "artifact for Session ${session_id} did not become available" >&2
  return 1
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
agent_id=$(first_item_value id)
[[ $agent_id == agt_lifecycle_complete ]]
await_cancel_agent_id=agt_lifecycle_await_cancel
await_approval_agent_id=agt_lifecycle_await_approval

complete_key="complete-${RANDOM}-${RANDOM}"
submit_session "$complete_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"complete\"}"
[[ $response_status == 201 ]]
complete_session=$(session_id)
[[ -n $complete_session ]]
complete_run=$(wait_latest_run_state "$complete_session" completed succeeded)
request POST "/api/copilot/v1/sessions/${complete_session}/events:ticket" '{}'
[[ $response_status == 200 && -n $(json_value ticket) && -n $(json_value websocket_url) ]]
artifact_id=$(wait_artifact "$complete_session")
request POST "/api/copilot/v1/artifacts/${artifact_id}:download" '{}'
[[ $response_status == 200 ]]
artifact_url=$(json_value download_url)
[[ -n $artifact_url ]]
artifact_content=$(curl --noproxy '*' -sS "$artifact_url")
[[ $artifact_content == 'Gantry deterministic artifact' ]]
assert_session_active "$complete_session"
submit_session "$complete_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"complete\"}"
[[ $response_status == 200 && $(session_id) == "$complete_session" ]]

approval_key="approval-${RANDOM}-${RANDOM}"
submit_session "$approval_key" "{\"agent_id\":\"${await_approval_agent_id}\",\"message\":\"write\"}"
[[ $response_status == 201 ]]
approval_session=$(session_id)
approval_run=$(wait_latest_run_state "$approval_session" awaiting_approval '')
[[ -n $approval_run ]]
request GET /api/copilot/v1/approvals
[[ $response_status == 200 ]]
approval_id=$(first_item_value id)
approval_digest=$(first_item_value action_digest)
approval_revision=$(first_item_number approval_revision)
[[ -n $approval_id && -n $approval_digest && -n $approval_revision ]]
request POST "/api/copilot/v1/approvals/${approval_id}:decide" "{\"decision\":\"approve\",\"action_digest\":\"${approval_digest}\",\"approval_revision\":${approval_revision}}" "$approval_key"
[[ $response_status == 200 ]]
wait_latest_run_state "$approval_session" completed succeeded "$approval_run" >/dev/null
assert_action_projection "$approval_run" "succeeded|true|printf approval|1|true"

rejection_key="rejection-${RANDOM}-${RANDOM}"
submit_session "$rejection_key" "{\"agent_id\":\"${await_approval_agent_id}\",\"message\":\"write\"}"
[[ $response_status == 201 ]]

rejection_session=$(session_id)
rejection_run=$(wait_latest_run_state "$rejection_session" awaiting_approval '')
[[ -n $rejection_run ]]
request GET /api/copilot/v1/approvals
[[ $response_status == 200 ]]
rejection_id=$(first_item_value id)
rejection_digest=$(first_item_value action_digest)
rejection_revision=$(first_item_number approval_revision)
[[ -n $rejection_id && -n $rejection_digest && -n $rejection_revision ]]
request POST "/api/copilot/v1/approvals/${rejection_id}:decide" "{\"decision\":\"reject\",\"action_digest\":\"${rejection_digest}\",\"approval_revision\":${rejection_revision}}" "$rejection_key"
[[ $response_status == 200 ]]
wait_latest_run_state "$rejection_session" completed requester_input_required "$rejection_run" >/dev/null
assert_session_active "$rejection_session"
assert_action_projection "$rejection_run" "rejected|true|printf approval|0|false"

other_token=$(token_for copilot-other gantry_other_password)
access_token=$other_token
request GET "/api/copilot/v1/sessions/${complete_session}"
[[ $response_status == 404 ]]
access_token=$(token_for copilot-demo gantry_demo_password)

cancel_key="cancel-${RANDOM}-${RANDOM}"
submit_session "$cancel_key" "{\"agent_id\":\"${await_cancel_agent_id}\",\"message\":\"wait\"}"
[[ $response_status == 201 ]]
cancel_session=$(session_id)
cancel_run=$(wait_latest_run_state "$cancel_session" running '')
request POST "/api/copilot/v1/sessions/${cancel_session}/runs/${cancel_run}:cancel" '{}' "$cancel_key"
[[ $response_status == 200 || $response_status == 202 ]]
wait_latest_run_state "$cancel_session" canceled canceled "$cancel_run" >/dev/null

loss_key="loss-${RANDOM}-${RANDOM}"
submit_session "$loss_key" "{\"agent_id\":\"${await_cancel_agent_id}\",\"message\":\"loss\"}"
loss_session=$(session_id)
loss_run=$(wait_latest_run_state "$loss_session" running '')
"${compose[@]}" kill runner
wait_latest_run_state "$loss_session" failed failed "$loss_run" >/dev/null
"${compose[@]}" up --detach runner

restart_key="restart-${RANDOM}-${RANDOM}"
submit_session "$restart_key" "{\"agent_id\":\"${await_cancel_agent_id}\",\"message\":\"restart\"}"
restart_session=$(session_id)
restart_run=$(wait_latest_run_state "$restart_session" running '')
"${compose[@]}" restart control-plane
wait_latest_run_state "$restart_session" failed failed "$restart_run" >/dev/null
request GET "/api/copilot/v1/sessions/${restart_session}"
[[ $response_status == 200 && $(session_state) == active && -n $response_etag ]]
retry_key="retry-${RANDOM}-${RANDOM}"
request POST "/api/copilot/v1/sessions/${restart_session}/runs/${restart_run}:retry" '{"revision_selection":"original_revision"}' "$retry_key" "$response_etag"
[[ $response_status == 201 ]]
retry_run=$(run_id)
[[ -n $retry_run ]]
wait_latest_run_state "$restart_session" running '' "$retry_run" >/dev/null
retry_cancel_key="retry-cancel-${RANDOM}-${RANDOM}"
request POST "/api/copilot/v1/sessions/${restart_session}/runs/${retry_run}:cancel" '{}' "$retry_cancel_key"
[[ $response_status == 200 || $response_status == 202 ]]
wait_latest_run_state "$restart_session" canceled canceled "$retry_run" >/dev/null

echo "Copilot persistent lifecycle smoke test passed."
