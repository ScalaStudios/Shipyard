#!/usr/bin/env bash
# Smoke-test a running shipyard-server against the public HTTP API.
# Usage: SHIPYARD_URL=http://127.0.0.1:8080 ./scripts/smoke.sh
set -euo pipefail

URL="${SHIPYARD_URL:-http://127.0.0.1:8080}"
COOKIE="$(mktemp)"
trap 'rm -f "$COOKIE"' EXIT

echo "==> health"
curl -fsS "$URL/healthz" >/dev/null
curl -fsS "$URL/readyz" >/dev/null
curl -fsS "$URL/api/v1/system/info" | grep -q shipyard

echo "==> bootstrap register/login"
USER="smoke$(date +%s)"
PASS='SmokeTest1!'
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"email\":\"$USER@example.com\",\"password\":\"$PASS\"}" \
  "$URL/api/v1/auth/register" >/dev/null || \
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' \
  -d "{\"login\":\"$USER\",\"password\":\"$PASS\"}" \
  "$URL/api/v1/auth/login" >/dev/null

curl -fsS -b "$COOKIE" "$URL/api/v1/me" | grep -q "$USER"

echo "==> org/project/pipeline"
ORG=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d "{\"slug\":\"smoke-org\",\"name\":\"Smoke Org\"}" \
  "$URL/api/v1/orgs")
ORG_ID=$(printf '%s' "$ORG" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)
PROJ=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d "{\"slug\":\"smoke-app\",\"name\":\"Smoke App\"}" \
  "$URL/api/v1/orgs/$ORG_ID/projects")
PROJ_ID=$(printf '%s' "$PROJ" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)

YAML=$'pipeline:\n  name: smoke\njobs:\n  greet:\n    runner:\n      os: linux\n    steps:\n      - name: echo\n        run: echo smoke\n'
PIPE=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  --data-binary "{\"slug\":\"smoke\",\"yaml\":$(printf '%s' "$YAML" | python -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}" \
  "$URL/api/v1/orgs/$ORG_ID/projects/$PROJ_ID/pipelines")
PIPE_ID=$(printf '%s' "$PIPE" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)

RUN=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{}' \
  "$URL/api/v1/orgs/$ORG_ID/projects/$PROJ_ID/pipelines/$PIPE_ID/runs")
printf '%s\n' "$RUN" | grep -q '"status"'

echo "==> package repo + maven metadata path"
PKG=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d '{"name":"libs","format":"maven"}' \
  "$URL/api/v1/orgs/$ORG_ID/projects/$PROJ_ID/packages")
printf '%s\n' "$PKG" | grep -q maven

echo "==> oidc providers endpoint"
curl -fsS "$URL/api/v1/auth/oidc/providers" | grep -q providers

echo "OK smoke against $URL"
