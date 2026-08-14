#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repository_root=$(cd -- "${script_dir}/../.." && pwd)
environment_file=${GANTRY_LOCAL_ENV_FILE:-"${repository_root}/.env.local"}
work_directory=$(mktemp -d)
control_plane_log="${work_directory}/control-plane.log"
runner_log="${work_directory}/runner.log"
control_plane_pid=""
runner_pid=""
response_body=""
response_status=""

if [[ -f ${environment_file} ]]; then
  set -a
  # The file is developer-controlled and uses the same KEY=value syntax as .env.example.
  source "${environment_file}"
  set +a
fi

export GANTRY_DEVELOPMENT_MODE=${GANTRY_DEVELOPMENT_MODE:-true}
export GANTRY_PHASE0_DEV_API_TOKEN=${GANTRY_PHASE0_DEV_API_TOKEN:-gantry_phase0_dev_token}
export GANTRY_CONTROL_PLANE_ADDR=${GANTRY_CONTROL_PLANE_ADDR:-"http://127.0.0.1:${GANTRY_GRPC_PORT:-8081}"}
export GANTRY_RUNNER_ID=${GANTRY_RUNNER_ID:-native-runner-01}

stop_process() {
  local pid=$1
  if [[ -n ${pid} ]] && kill -0 "${pid}" 2>/dev/null; then
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  fi
}

cleanup() {
  local status=$?
  stop_process "${runner_pid}"
  stop_process "${control_plane_pid}"
  if [[ ${status} -ne 0 ]]; then
    cat "${control_plane_log}" >&2 || true
    cat "${runner_log}" >&2 || true
  fi
  rm -rf "${work_directory}"
  exit "${status}"
}
trap cleanup EXIT

require_command() {
  if ! command -v "$1" >/dev/null; then
    echo "${1} is required for the native Phase 0 smoke test" >&2
    return 1
  fi
}

request() {
  local method=$1 path=$2 payload=${3:-} output
  output=$(mktemp)
  local args=(-sS -o "${output}" -w '%{http_code}' -X "${method}")
  if [[ ${path} == /internal/* ]]; then
    args+=(-H "Authorization: Bearer ${GANTRY_PHASE0_DEV_API_TOKEN}")
  fi
  if [[ -n ${payload} ]]; then
    args+=(-H 'Content-Type: application/json' --data "${payload}")
  fi
  if ! response_status=$(curl "${args[@]}" "http://127.0.0.1:${GANTRY_HTTP_PORT:-8080}${path}"); then
    response_status=000
  fi
  response_body=$(<"${output}")
  rm -f "${output}"
}

json_value() {
  sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p" <<<"${response_body}" | head -n1
}

wait_for_status() {
  local run_id=$1 expected=$2
  for _ in $(seq 1 80); do
    request GET "/internal/phase0/runs/${run_id}"
    if [[ ${response_status} == 200 && $(json_value status) == "${expected}" ]]; then
      return 0
    fi
    sleep 0.25
  done
  echo "run ${run_id} did not reach ${expected}: HTTP ${response_status} ${response_body}" >&2
  return 1
}

wait_for_ready() {
  for _ in $(seq 1 80); do
    request GET /readyz
    if [[ ${response_status} == 200 ]]; then
      return 0
    fi
    sleep 0.25
  done
  echo "control plane did not become ready: HTTP ${response_status} ${response_body}" >&2
  return 1
}

start_run() {
  local mode=$1
  for _ in $(seq 1 80); do
    request POST /internal/phase0/runs "{\"mode\":\"${mode}\"}"
    if [[ ${response_status} == 201 ]]; then
      json_value run_id
      return 0
    fi
    if [[ ${response_status} != 409 ]]; then
      echo "could not create ${mode} run: HTTP ${response_status} ${response_body}" >&2
      return 1
    fi
    sleep 0.25
  done
  echo "runner did not become available" >&2
  return 1
}

require_command curl
require_command go
require_command cargo

if [[ ${GANTRY_DEVELOPMENT_MODE} != true ]]; then
  echo "GANTRY_DEVELOPMENT_MODE must be true for the native Phase 0 smoke test" >&2
  exit 1
fi
if [[ -z ${GANTRY_PHASE0_DEV_API_TOKEN} ]]; then
  echo "GANTRY_PHASE0_DEV_API_TOKEN must be set for the native Phase 0 smoke test" >&2
  exit 1
fi

(
  cd "${repository_root}/control-plane"
  go build -o "${work_directory}/gantry" ./cmd/gantry
)
(
  cd "${repository_root}/runner"
  cargo build --bin runner
)

runner_binary="${repository_root}/runner/target/debug/runner"
if [[ -f "${runner_binary}.exe" ]]; then
  runner_binary="${runner_binary}.exe"
fi
if [[ ! -f "${runner_binary}" ]]; then
  echo "runner binary was not built at ${runner_binary}" >&2
  exit 1
fi

"${work_directory}/gantry" >"${control_plane_log}" 2>&1 &
control_plane_pid=$!
wait_for_ready
"${runner_binary}" >"${runner_log}" 2>&1 &
runner_pid=$!

complete_run=$(start_run complete)
wait_for_status "${complete_run}" completed

cancel_run=$(start_run await_cancel)
wait_for_status "${cancel_run}" running
request POST "/internal/phase0/runs/${cancel_run}/cancel"
[[ ${response_status} == 202 ]]
wait_for_status "${cancel_run}" canceled

lost_runner_run=$(start_run await_cancel)
wait_for_status "${lost_runner_run}" running
stop_process "${runner_pid}"
runner_pid=""
wait_for_status "${lost_runner_run}" failed

echo "Native Phase 0 lifecycle smoke test passed."
