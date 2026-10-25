#!/bin/sh
# Checks the Kinlight image before it goes to the homelab, on this machine,
# with Docker: it builds and starts the image with a local S3 server
# (compose.test.yaml), checks production mode, then deletes the database
# volume and checks that the next start restores it from the backup.
#
#   deploy/check.sh
#
# It uses only test values and leaves nothing running.
set -eu
cd "$(dirname "$0")/.."

project=kinlight-check
compose() { docker compose -p "$project" -f compose.yaml -f compose.test.yaml "$@"; }
base=http://127.0.0.1:18080
email=parent@check.test
password="check-$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
failed=0

pass() { echo "pass  $1"; }
fail() {
  echo "FAIL  $1"
  failed=1
}
wait_healthy() {
  for _ in $(seq 1 60); do
    if curl -sf "$base/api/health" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "Kinlight didn't become healthy; its log:" >&2
  compose logs kinlight >&2
  exit 1
}
status() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
cleanup() { compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT

# Placeholders for compose.yaml's required settings; compose.test.yaml
# replaces them with its own test values.
export KINLIGHT_PUBLIC_URL=https://kinlight.check.test OPENROUTER_API_KEY=x BRAVE_API_KEY=x \
  LITESTREAM_BUCKET=x LITESTREAM_ACCESS_KEY_ID=x LITESTREAM_SECRET_ACCESS_KEY=x

cleanup
echo "Building and starting (this takes a minute the first time)…"
compose up -d --build --quiet-pull >/dev/null 2>&1 || compose up -d --build
wait_healthy

# The app, its files and production's headers.
headers=$(curl -s -D - -o /dev/null "$base/login")
case "$headers" in *"Content-Security-Policy: default-src 'self'; script-src 'self' 'sha256-"*) pass "the page has the CSP, with the theme script's hash" ;; *) fail "the page's CSP" ;; esac
case "$headers" in *"Strict-Transport-Security"*) pass "HSTS is sent" ;; *) fail "HSTS" ;; esac
asset=$(curl -s "$base/login" | grep -o '/assets/[^"]*\.js' | head -1)
case "$(curl -s -D - -o /dev/null "$base$asset")" in *"immutable"*) pass "assets are cached for good ($asset)" ;; *) fail "the asset's cache header" ;; esac
[ "$(status "$base/api/nothing-here")" = 404 ] && pass "an unknown API path is a 404" || fail "the API 404"

# Sign-up: the first account only.
json='{"email":"'$email'","password":"'$password'"}'
[ "$(status -H 'Content-Type: application/json' -d "$json" "$base/api/signup")" = 201 ] && pass "the first sign-up creates the family" || fail "the first sign-up"
[ "$(status -H 'Content-Type: application/json' -d '{"email":"stranger@check.test","password":"a long enough password"}' "$base/api/signup")" = 403 ] && pass "a second sign-up is refused" || fail "the second sign-up"

# Backup and restore: give Litestream time to send the new rows, then lose
# the volume and start again.
sleep 15
compose stop kinlight >/dev/null 2>&1
compose rm -f kinlight >/dev/null 2>&1
docker volume rm "${project}_kinlight-data" >/dev/null
compose up -d --no-deps kinlight >/dev/null 2>&1
wait_healthy
compose logs kinlight 2>/dev/null | grep -q "restoring from the backup" && pass "the next start restored from the backup" || fail "no restore on start"
[ "$(status -H 'Content-Type: application/json' -d "$json" "$base/api/login")" = 200 ] && pass "the account came back: it logs in" || fail "the account after the restore"
curl -s "$base/api/signup" | grep -q '"open":false' && pass "sign-up is still closed after the restore" || fail "sign-up after the restore"

if [ "$failed" -ne 0 ]; then
  echo "Some checks failed; Kinlight's log:"
  compose logs kinlight | tail -40
  exit 1
fi
echo "All checks passed."
