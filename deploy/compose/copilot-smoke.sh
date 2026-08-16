#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -p gantry-copilot-smoke -f docker-compose.yml)
api_url="${GANTRY_COPILOT_SMOKE_API_URL:-http://localhost:8080}"
dex_url="${GANTRY_DEX_SMOKE_URL:-http://localhost:5556/dex}"
response_body=""
response_status=""
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
task_id() { sed -n 's/^{"id":"\([^"]*\)".*/\1/p' <<<"$response_body"; }
task_run_id() { sed -n 's/.*"current_run":{"id":"\([^"]*\)".*/\1/p' <<<"$response_body"; }

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
  local method=$1 path=$2 payload=${3:-} output
  output=$(mktemp)
  local args=(-sS -o "$output" -w '%{http_code}' -X "$method" -H "Authorization: Bearer ${access_token}")
  if [[ -n $payload ]]; then args+=(-H 'Content-Type: application/json' --data "$payload"); fi
  response_status=$(curl --noproxy '*' "${args[@]}" "${api_url}${path}")
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
  response_status=$(curl --noproxy '*' -sS -o "$output" -w '%{http_code}' -X POST "${api_url}/api/copilot/v1/tasks" \
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
  local task_id=$1
  for _ in $(seq 1 80); do
    local value
    value=$("${compose[@]}" exec -T postgres psql -U gantry -d gantry -Atqc "SELECT id FROM gantry.artifacts WHERE task_id='${task_id}' AND state='available' AND scan_status='passed' LIMIT 1")
    [[ -n $value ]] && { printf '%s\n' "$value"; return 0; }
    sleep 0.25
  done
  echo "artifact for task ${task_id} did not become available" >&2
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
agent_id=$(json_value id)
[[ $agent_id == agt_lifecycle_complete ]]
await_cancel_agent_id=agt_lifecycle_await_cancel
await_approval_agent_id=agt_lifecycle_await_approval

complete_key="complete-${RANDOM}-${RANDOM}"
submit "$complete_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"complete\"}"
[[ $response_status == 201 ]]
complete_task=$(task_id)
[[ -n $complete_task ]]
wait_task_status "$complete_task" completed
request POST "/api/copilot/v1/tasks/${complete_task}/events:ticket" '{}'
[[ $response_status == 200 && -n $(json_value ticket) ]]
artifact_id=$(wait_artifact "$complete_task")
request POST "/api/copilot/v1/artifacts/${artifact_id}:download" '{}'
[[ $response_status == 200 ]]
artifact_url=$(json_value download_url)
[[ -n $artifact_url ]]
artifact_content=$(curl --noproxy '*' -sS "$artifact_url")
[[ $artifact_content == 'Gantry deterministic artifact' ]]
submit "$complete_key" "{\"agent_id\":\"${agent_id}\",\"message\":\"complete\"}"
[[ $response_status == 200 && $(task_id) == "$complete_task" ]]

approval_key="approval-${RANDOM}-${RANDOM}"
submit "$approval_key" "{\"agent_id\":\"${await_approval_agent_id}\",\"message\":\"write\"}"
[[ $response_status == 201 ]]
approval_task=$(task_id)
wait_task_status "$approval_task" awaiting_approval
request GET "/api/copilot/v1/tasks/${approval_task}"
[[ $response_status == 200 ]]
approval_run=$(task_run_id)
[[ -n $approval_run ]]
request GET /api/copilot/v1/approvals
[[ $response_status == 200 ]]
approval_id=$(json_value id)
approval_digest=$(json_value action_digest)
[[ -n $approval_id && -n $approval_digest ]]
request POST "/api/copilot/v1/approvals/${approval_id}:decide" "{\"decision\":\"approve\",\"action_digest\":\"${approval_digest}\",\"idempotency_key\":\"${approval_key}\"}"
[[ $response_status == 200 ]]
wait_task_status "$approval_task" completed
assert_action_projection "$approval_run" "succeeded|true|printf approval|1|true"

rejection_key="rejection-${RANDOM}-${RANDOM}"
submit "$rejection_key" "{\"agent_id\":\"${await_approval_agent_id}\",\"message\":\"write\"}"
[[ $response_status == 201 ]]

rejection_task=$(task_id)
wait_task_status "$rejection_task" awaiting_approval
request GET "/api/copilot/v1/tasks/${rejection_task}"
[[ $response_status == 200 ]]
rejection_run=$(task_run_id)
[[ -n $rejection_run ]]
request GET /api/copilot/v1/approvals
[[ $response_status == 200 ]]
rejection_id=$(json_value id)
rejection_digest=$(json_value action_digest)
[[ -n $rejection_id && -n $rejection_digest ]]
request POST "/api/copilot/v1/approvals/${rejection_id}:decide" "{\"decision\":\"reject\",\"action_digest\":\"${rejection_digest}\",\"idempotency_key\":\"${rejection_key}\"}"
[[ $response_status == 200 ]]
wait_task_status "$rejection_task" failed
assert_action_projection "$rejection_run" "rejected|true|printf approval|0|false"

other_token=$(token_for copilot-other gantry_other_password)
access_token=$other_token
request GET "/api/copilot/v1/tasks/${complete_task}"
[[ $response_status == 404 ]]
access_token=$(token_for copilot-demo gantry_demo_password)

cancel_key="cancel-${RANDOM}-${RANDOM}"
submit "$cancel_key" "{\"agent_id\":\"${await_cancel_agent_id}\",\"message\":\"wait\"}"
[[ $response_status == 201 ]]
cancel_task=$(task_id)
cancel_run=$(task_run_id)
wait_task_status "$cancel_task" running
request POST "/api/copilot/v1/tasks/${cancel_task}/runs/${cancel_run}:cancel" '{}'
[[ $response_status == 200 ]]
wait_task_status "$cancel_task" canceled

loss_key="loss-${RANDOM}-${RANDOM}"
submit "$loss_key" "{\"agent_id\":\"${await_cancel_agent_id}\",\"message\":\"loss\"}"
loss_task=$(task_id)
wait_task_status "$loss_task" running
"${compose[@]}" kill runner
wait_task_status "$loss_task" failed
"${compose[@]}" up --detach runner

restart_key="restart-${RANDOM}-${RANDOM}"
submit "$restart_key" "{\"agent_id\":\"${await_cancel_agent_id}\",\"message\":\"restart\"}"
restart_task=$(task_id)
wait_task_status "$restart_task" running
"${compose[@]}" restart control-plane
wait_task_status "$restart_task" failed
request POST "/api/copilot/v1/tasks/${restart_task}:retry" '{}'
[[ $response_status == 201 ]]
retry_run=$(task_run_id)
[[ -n $retry_run ]]
wait_task_status "$restart_task" running
request POST "/api/copilot/v1/tasks/${restart_task}/runs/${retry_run}:cancel" '{}'
[[ $response_status == 200 ]]
wait_task_status "$restart_task" canceled

echo "Copilot persistent lifecycle smoke test passed."
