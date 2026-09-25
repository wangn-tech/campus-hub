#!/usr/bin/env bash
set -Eeuo pipefail

compose=(docker compose -f deploy/docker/docker-compose.yml --env-file .env.example)
temporary_dir="$(mktemp -d)"
server_pid=""
integration_port="${CAMPUSHUB_INTEGRATION_PORT:-}"
if [[ -z "${integration_port}" ]]; then
  for candidate_port in $(seq 18081 18120); do
    if ! ss -H -ltn "sport = :${candidate_port}" | grep -q .; then
      integration_port="${candidate_port}"
      break
    fi
  done
fi
if [[ -z "${integration_port}" ]]; then
  echo "No free integration port in range 18081-18120" >&2
  exit 1
fi

cleanup() {
  status=$?
  if [[ -n "${server_pid}" ]] && kill -0 "${server_pid}" 2>/dev/null; then
    kill -TERM "${server_pid}" 2>/dev/null || true
    wait "${server_pid}" || true
  fi
  if [[ ${status} -ne 0 ]]; then
    cat "${temporary_dir}/server.log" 2>/dev/null || true
    "${compose[@]}" logs --no-color || true
  fi
  if [[ "${CAMPUSHUB_CI:-}" == "1" ]]; then
    "${compose[@]}" down --volumes --remove-orphans || true
  else
    "${compose[@]}" down --remove-orphans || true
  fi
  rm -rf "${temporary_dir}"
  exit "${status}"
}
trap cleanup EXIT

wait_for_status() {
  expected_status="$1"
  path="$2"
  for _ in $(seq 1 60); do
    status="$(curl --silent --show-error --output "${temporary_dir}/response.json" --write-out '%{http_code}' "http://127.0.0.1:${integration_port}${path}" || true)"
    if [[ "${status}" == "${expected_status}" ]]; then
      return 0
    fi
    sleep 1
  done
  cat "${temporary_dir}/response.json" 2>/dev/null || true
  return 1
}

echo "Starting Compose dependencies"
"${compose[@]}" up --detach --wait --wait-timeout 180 mysql redis kafka elasticsearch
echo "Applying migrations"
go run ./cmd/migrate -direction up
echo "Building and starting backend"
go build -o "${temporary_dir}/campushub" ./cmd/server
CAMPUSHUB_HTTP_PORT="${integration_port}" "${temporary_dir}/campushub" >"${temporary_dir}/server.log" 2>&1 &
server_pid=$!

wait_for_status 200 /health
wait_for_status 200 /ready

echo "Verifying registration and login"
email="integration-$(date +%s%N)@example.com"
register_status="$(curl --silent --show-error --output "${temporary_dir}/register.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/auth/register" --header 'Content-Type: application/json' --data "{\"email\":\"${email}\",\"password\":\"integration-password\",\"nickname\":\"integration\"}")"
[[ "${register_status}" == "200" ]]
login_status="$(curl --silent --show-error --output "${temporary_dir}/login.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/auth/login" --header 'Content-Type: application/json' --data "{\"email\":\"${email}\",\"password\":\"integration-password\"}")"
[[ "${login_status}" == "200" ]]
grep -q 'access_token' "${temporary_dir}/login.json"

echo "Verifying Redis readiness recovery"
"${compose[@]}" stop redis
wait_for_status 503 /ready
wait_for_status 200 /health
"${compose[@]}" start redis
"${compose[@]}" up --detach --wait --wait-timeout 60 redis
wait_for_status 200 /ready
echo "Integration test passed"
