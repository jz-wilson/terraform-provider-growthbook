#!/usr/bin/env bash
# Bootstrap a fresh GrowthBook instance and print a secret API key.
#
# Creates the first organization and user through the unauthenticated
# /auth/firsttime endpoint, then mints an API key for that organization.
# Safe to re-run: if the instance already has an organization, it logs in
# with the same credentials instead.
#
# Usage: export GROWTHBOOK_API_KEY=$(e2e/bootstrap.sh)
set -euo pipefail

BASE_URL="${GROWTHBOOK_BASE_URL:-http://localhost:3100}"
EMAIL="${GROWTHBOOK_E2E_EMAIL:-e2e@example.com}"
PASSWORD="${GROWTHBOOK_E2E_PASSWORD:-e2e-password-1}"

log() { echo "bootstrap: $*" >&2; }

wait_for_api() {
  for _ in $(seq 1 120); do
    if curl -fsS "${BASE_URL}/healthcheck" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  log "GrowthBook API at ${BASE_URL} did not become healthy"
  return 1
}

# post PATH JSON [extra curl args...] -> body
post() {
  local path="$1" body="$2"
  shift 2
  curl -fsS -X POST "${BASE_URL}${path}" -H "Content-Type: application/json" "$@" --data-raw "${body}"
}

wait_for_api

token=""
firsttime_payload=$(printf '{"companyname":"e2e","name":"e2e","email":"%s","password":"%s"}' "$EMAIL" "$PASSWORD")
if out=$(post /auth/firsttime "$firsttime_payload" 2>/dev/null); then
  token=$(echo "$out" | jq -r '.token // empty')
  log "created first organization"
fi
if [ -z "$token" ]; then
  login_payload=$(printf '{"email":"%s","password":"%s"}' "$EMAIL" "$PASSWORD")
  token=$(post /auth/login "$login_payload" | jq -r '.token // empty')
  log "logged in to existing organization"
fi
[ -n "$token" ] || { log "could not obtain a session token"; exit 1; }

org_id=$(curl -fsS "${BASE_URL}/user" -H "Authorization: Bearer ${token}" | jq -r '.organizations[0].id')
[ -n "$org_id" ] && [ "$org_id" != "null" ] || { log "no organization on user"; exit 1; }

apikey=$(post /keys '{"description":"e2e","type":"user"}' \
  -H "X-Organization: ${org_id}" -H "Authorization: Bearer ${token}" | jq -r '.key.key // empty')
case "$apikey" in
  secret_*) ;;
  *) log "unexpected API key shape: '${apikey:0:8}...'"; exit 1 ;;
esac

log "minted API key for organization ${org_id}"
printf '%s\n' "$apikey"
