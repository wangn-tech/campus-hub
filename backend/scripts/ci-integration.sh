#!/usr/bin/env bash
set -Eeuo pipefail

compose=(docker compose -f deploy/docker/docker-compose.yml --env-file .env.example)
temporary_dir="$(mktemp -d)"
server_pid=""
mysql_user="${CAMPUSHUB_MYSQL_USERNAME:-campushub}"
mysql_password="${CAMPUSHUB_MYSQL_PASSWORD:-change-me}"
mysql_database="${CAMPUSHUB_MYSQL_DATABASE:-campushub}"
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

# create_published_activity creates an activity whose registration window is
# open and approves it, so it can be registered for. Prints the activity id.
create_published_activity() {
  local require_approval="$1" title="$2" payload id
  payload="$(printf '{"title":"%s","description":"created by ci","category_id":"%s","contact_phone":"13800000000","register_start_at":%s,"register_end_at":%s,"activity_start_at":%s,"activity_end_at":%s,"location":"CI Hall","address_detail":"integration","max_participants":30,"require_approval":%s,"require_student_verify":false,"min_credit_score":0,"tag_ids":["%s"],"is_draft":false}' \
    "${title}" "${category_id}" "$(( now_ms - hour_ms ))" "$(( now_ms + hour_ms ))" "$(( now_ms + 2 * hour_ms ))" "$(( now_ms + 3 * hour_ms ))" "${require_approval}" "${tag_id}")"
  curl --silent --show-error --output "${temporary_dir}/helper-activity.json" --request POST "http://127.0.0.1:${integration_port}/api/v1/activities" --header "Authorization: Bearer ${access_token}" --header 'Content-Type: application/json' --data "${payload}"
  id="$(sed -n 's/.*"data":{"id":"\([^"]*\)".*/\1/p' "${temporary_dir}/helper-activity.json")"
  [[ -n "${id}" ]]
  curl --silent --show-error --output /dev/null --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${id}/submit" --header "Authorization: Bearer ${access_token}"
  curl --silent --show-error --output /dev/null --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${id}/approve" --header "Authorization: Bearer ${access_token}"
  printf '%s' "${id}"
}

# register_user registers a fresh account via the email code flow and sets the
# globals `email` and `access_token`. It must be called directly (not inside a
# command substitution) so those assignments survive.
register_user() {
  local label="$1" code
  email="integration-${label}-$(date +%s%N)@example.com"
  # Clear the mailbox so the poll below cannot pick up an earlier code.
  curl --silent --output /dev/null --request DELETE "http://127.0.0.1:18025/api/v1/messages" || true
  curl --silent --show-error --output "${temporary_dir}/email-code.json" --request POST "http://127.0.0.1:${integration_port}/api/v1/email-codes" --header 'Content-Type: application/json' --data "{\"email\":\"${email}\",\"scene\":\"register\"}"
  for _ in $(seq 1 30); do
    curl --silent "http://127.0.0.1:18025/api/v1/messages" >"${temporary_dir}/mailpit.json" || true
    code="$(grep -Eo '[0-9]{6}' "${temporary_dir}/mailpit.json" | tail -n 1 || true)"
    [[ -n "${code}" ]] && break
    sleep 1
  done
  [[ -n "${code}" ]]
  curl --silent --show-error --output "${temporary_dir}/register.json" --request POST "http://127.0.0.1:${integration_port}/api/v1/auth/register" --header 'Content-Type: application/json' --data "{\"email\":\"${email}\",\"password\":\"integration-password\",\"nickname\":\"integration\",\"code\":\"${code}\"}"
  curl --silent --show-error --output "${temporary_dir}/login.json" --request POST "http://127.0.0.1:${integration_port}/api/v1/auth/login" --header 'Content-Type: application/json' --data "{\"email\":\"${email}\",\"password\":\"integration-password\"}"
  access_token="$(sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p' "${temporary_dir}/login.json")"
  [[ -n "${access_token}" ]]
}

echo "Starting Compose dependencies"
# Start the whole stack without waiting so image pulls and the slow
# Elasticsearch boot overlap with the flows below. Only MySQL, Redis, RustFS and
# Mailpit are needed by those flows; Kafka and Elasticsearch are waited for at
# the end, right before the /ready assertion.
"${compose[@]}" up --detach mysql redis kafka elasticsearch rustfs mailpit
"${compose[@]}" up --detach --wait --wait-timeout 180 mysql redis rustfs mailpit
echo "Applying migrations"
go run ./cmd/migrate -direction up
echo "Building and starting backend"
go build -o "${temporary_dir}/campushub" ./cmd/server
# Run the maintenance scheduler quickly so the tests can assert the time based
# transitions instead of waiting a full minute.
CAMPUSHUB_HTTP_PORT="${integration_port}" CAMPUSHUB_ACTIVITY_SCHEDULER_INTERVAL=5s "${temporary_dir}/campushub" >"${temporary_dir}/server.log" 2>&1 &
server_pid=$!

wait_for_status 200 /health

echo "Verifying registration and login"
register_user primary
[[ -n "${access_token}" ]]

echo "Verifying private RustFS upload and recovery"
printf 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' | base64 --decode >"${temporary_dir}/image.png"
upload_status="$(curl --silent --show-error --output "${temporary_dir}/upload.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/files/images" --header "Authorization: Bearer ${access_token}" --form "file=@${temporary_dir}/image.png;type=image/png" --form 'biz_type=avatar')"
[[ "${upload_status}" == "200" ]]
grep -q 'access_url' "${temporary_dir}/upload.json"
"${compose[@]}" stop rustfs
upload_down_status="$(curl --silent --show-error --output "${temporary_dir}/upload-down.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/files/images" --header "Authorization: Bearer ${access_token}" --form "file=@${temporary_dir}/image.png;type=image/png" --form 'biz_type=avatar' || true)"
[[ "${upload_down_status}" == "503" ]]
"${compose[@]}" start rustfs
"${compose[@]}" up --detach --wait --wait-timeout 60 rustfs
upload_recovered_status="$(curl --silent --show-error --output "${temporary_dir}/upload-recovered.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/files/images" --header "Authorization: Bearer ${access_token}" --form "file=@${temporary_dir}/image.png;type=image/png" --form 'biz_type=avatar')"
[[ "${upload_recovered_status}" == "200" ]]

echo "Verifying activity lifecycle"
category_id="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/categories" | grep -o '"id":"[^"]*"' | head -n 1 | cut -d'"' -f4)"
[[ -n "${category_id}" ]]
tag_id="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/tags?type=activity" | grep -o '"id":"[^"]*"' | head -n 1 | cut -d'"' -f4)"
[[ -n "${tag_id}" ]]
now_ms="$(( $(date +%s) * 1000 ))"
hour_ms=3600000
payload="$(printf '{"title":"integration activity","description":"created by ci","category_id":"%s","contact_phone":"13800000000","register_start_at":%s,"register_end_at":%s,"activity_start_at":%s,"activity_end_at":%s,"location":"CI Hall","address_detail":"integration","max_participants":30,"require_approval":false,"require_student_verify":false,"min_credit_score":0,"tag_ids":["%s"],"is_draft":true}' \
  "${category_id}" "$(( now_ms + hour_ms ))" "$(( now_ms + 2 * hour_ms ))" "$(( now_ms + 3 * hour_ms ))" "$(( now_ms + 4 * hour_ms ))" "${tag_id}")"
create_status="$(curl --silent --show-error --output "${temporary_dir}/activity-created.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities" --header "Authorization: Bearer ${access_token}" --header 'Content-Type: application/json' --data "${payload}")"
[[ "${create_status}" == "200" ]]
activity_id="$(grep -o '"id":"[^"]*"' "${temporary_dir}/activity-created.json" | head -n 1 | cut -d'"' -f4)"
[[ -n "${activity_id}" ]]
submit_status="$(curl --silent --show-error --output "${temporary_dir}/activity-submit.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${activity_id}/submit" --header "Authorization: Bearer ${access_token}")"
[[ "${submit_status}" == "200" ]]
denied_status="$(curl --silent --show-error --output "${temporary_dir}/activity-denied.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${activity_id}/approve" --header "Authorization: Bearer ${access_token}")"
[[ "${denied_status}" == "403" ]]
"${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" -e "INSERT IGNORE INTO user_roles (user_id, role_id, created_at) SELECT u.id, r.id, UTC_TIMESTAMP(3) FROM users u JOIN roles r ON r.code = 'admin' WHERE u.email = '${email}'" >/dev/null 2>&1
approve_status="$(curl --silent --show-error --output "${temporary_dir}/activity-approved.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${activity_id}/approve" --header "Authorization: Bearer ${access_token}")"
[[ "${approve_status}" == "200" ]]
list_body="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/activities?page=1&page_size=50")"
grep -q "${activity_id}" <<<"${list_body}"
search_body="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/activities/search?keyword=integration")"
grep -q "${activity_id}" <<<"${search_body}"
detail_status="$(curl --silent --show-error --output "${temporary_dir}/activity-detail.json" --write-out '%{http_code}' "http://127.0.0.1:${integration_port}/api/v1/activities/${activity_id}")"
[[ "${detail_status}" == "200" ]]
grep -q 'integration activity' "${temporary_dir}/activity-detail.json"
cancel_status="$(curl --silent --show-error --output "${temporary_dir}/activity-cancelled.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${activity_id}/cancel" --header "Authorization: Bearer ${access_token}")"
[[ "${cancel_status}" == "200" ]]
status_log_count="$("${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" --batch --skip-column-names -e "SELECT COUNT(*) FROM activity_status_logs WHERE activity_id = (SELECT id FROM activities WHERE uuid = '${activity_id}')" 2>/dev/null | tr -d '[:space:]')"
[[ "${status_log_count}" -ge 3 ]]

echo "Verifying registration lifecycle"
register_activity="$(create_published_activity false "integration register activity")"
register_status="$(curl --silent --show-error --output "${temporary_dir}/registration.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${register_activity}/registrations" --header "Authorization: Bearer ${access_token}")"
[[ "${register_status}" == "200" ]]
registration_id="$(sed -n 's/.*"data":{"id":"\([^"]*\)".*/\1/p' "${temporary_dir}/registration.json")"
ticket_id="$(sed -n 's/.*"ticket":{"id":"\([^"]*\)".*/\1/p' "${temporary_dir}/registration.json")"
[[ -n "${registration_id}" && -n "${ticket_id}" ]]
grep -q '"status":1' "${temporary_dir}/registration.json"
duplicate_status="$(curl --silent --show-error --output "${temporary_dir}/registration-duplicate.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${register_activity}/registrations" --header "Authorization: Bearer ${access_token}")"
[[ "${duplicate_status}" == "409" ]]
my_registrations="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/users/me/activities/registered?page=1&page_size=50" --header "Authorization: Bearer ${access_token}")"
grep -q "${registration_id}" <<<"${my_registrations}"
my_tickets="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/tickets?page=1&page_size=50" --header "Authorization: Bearer ${access_token}")"
grep -q "${ticket_id}" <<<"${my_tickets}"
cancel_registration_status="$(curl --silent --show-error --output "${temporary_dir}/registration-cancelled.json" --write-out '%{http_code}' --request DELETE "http://127.0.0.1:${integration_port}/api/v1/registrations/${registration_id}" --header "Authorization: Bearer ${access_token}")"
[[ "${cancel_registration_status}" == "200" ]]
ticket_detail="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/tickets/${ticket_id}" --header "Authorization: Bearer ${access_token}")"
grep -q '"status":3' <<<"${ticket_detail}"
registration_log_count="$("${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" --batch --skip-column-names -e "SELECT COUNT(*) FROM registration_status_logs WHERE registration_id = (SELECT id FROM activity_registrations WHERE uuid = '${registration_id}')" 2>/dev/null | tr -d '[:space:]')"
[[ "${registration_log_count}" -ge 1 ]]
registration_event_count="$("${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" --batch --skip-column-names -e "SELECT COUNT(*) FROM outbox_events WHERE event_type IN ('registration.created','ticket.created','registration.cancelled','ticket.voided')" 2>/dev/null | tr -d '[:space:]')"
[[ "${registration_event_count}" -ge 4 ]]

echo "Verifying registration approval"
approval_activity="$(create_published_activity true "integration approval activity")"
pending_status="$(curl --silent --show-error --output "${temporary_dir}/registration-pending.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${approval_activity}/registrations" --header "Authorization: Bearer ${access_token}")"
[[ "${pending_status}" == "200" ]]
pending_registration="$(sed -n 's/.*"data":{"id":"\([^"]*\)".*/\1/p' "${temporary_dir}/registration-pending.json")"
[[ -n "${pending_registration}" ]]
grep -q '"status":0' "${temporary_dir}/registration-pending.json"
if grep -q '"ticket"' "${temporary_dir}/registration-pending.json"; then
  echo "a pending registration must not carry a ticket" >&2
  exit 1
fi
# register_user overwrites access_token, so keep the organizer token around and
# restore it after creating the second account.
primary_token="${access_token}"
register_user other
other_token="${access_token}"
access_token="${primary_token}"
[[ -n "${other_token}" ]]
foreign_status="$(curl --silent --show-error --output "${temporary_dir}/registration-foreign.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/registrations/${pending_registration}/approve" --header "Authorization: Bearer ${other_token}")"
[[ "${foreign_status}" == "403" ]]
approve_status="$(curl --silent --show-error --output "${temporary_dir}/registration-approved.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/registrations/${pending_registration}/approve" --header "Authorization: Bearer ${access_token}")"
[[ "${approve_status}" == "200" ]]
approved_detail="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/registrations/${pending_registration}" --header "Authorization: Bearer ${access_token}")"
grep -q '"status":1' <<<"${approved_detail}"
grep -q '"ticket"' <<<"${approved_detail}"
activity_registrations="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/activities/${approval_activity}/registrations?status=approved&page=1&page_size=50" --header "Authorization: Bearer ${access_token}")"
grep -q "${pending_registration}" <<<"${activity_registrations}"
grep -q '"nickname":"integration"' <<<"${activity_registrations}"

echo "Verifying approval timeout"
expiry_activity="$(create_published_activity true "integration expiry activity")"
expiry_register_status="$(curl --silent --show-error --output "${temporary_dir}/registration-expiry.json" --write-out '%{http_code}' --request POST "http://127.0.0.1:${integration_port}/api/v1/activities/${expiry_activity}/registrations" --header "Authorization: Bearer ${access_token}")"
[[ "${expiry_register_status}" == "200" ]]
expiry_registration="$(sed -n 's/.*"data":{"id":"\([^"]*\)".*/\1/p' "${temporary_dir}/registration-expiry.json")"
[[ -n "${expiry_registration}" ]]
"${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" -e "UPDATE activity_registrations SET expires_at = UTC_TIMESTAMP(3) - INTERVAL 1 MINUTE WHERE uuid = '${expiry_registration}'" >/dev/null 2>&1
expired_detail=""
for _ in $(seq 1 20); do
  expired_detail="$(curl --silent --show-error "http://127.0.0.1:${integration_port}/api/v1/registrations/${expiry_registration}" --header "Authorization: Bearer ${access_token}")"
  if grep -q '"status":5' <<<"${expired_detail}"; then
    break
  fi
  sleep 1
done
grep -q '"status":5' <<<"${expired_detail}"
pending_slots="$("${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" --batch --skip-column-names -e "SELECT pending_participant_count FROM activities WHERE uuid = '${expiry_activity}'" 2>/dev/null | tr -d '[:space:]')"
[[ "${pending_slots}" == "0" ]]
expired_events="$("${compose[@]}" exec -T mysql mysql --user="${mysql_user}" --password="${mysql_password}" "${mysql_database}" --batch --skip-column-names -e "SELECT COUNT(*) FROM outbox_events WHERE event_type = 'registration.expired'" 2>/dev/null | tr -d '[:space:]')"
[[ "${expired_events}" -ge 1 ]]

echo "Waiting for Kafka and Elasticsearch readiness"
"${compose[@]}" up --detach --wait --wait-timeout 180 kafka elasticsearch
wait_for_status 200 /ready

echo "Verifying Redis readiness recovery"
"${compose[@]}" stop redis
wait_for_status 503 /ready
wait_for_status 200 /health
"${compose[@]}" start redis
"${compose[@]}" up --detach --wait --wait-timeout 60 redis
wait_for_status 200 /ready
echo "Integration test passed"
