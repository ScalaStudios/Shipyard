#!/usr/bin/env bash
# Smoke-test a running shipyard-server against the public HTTP API.
# Usage: SHIPYARD_URL=http://127.0.0.1:8080 ./scripts/smoke.sh
set -euo pipefail

URL="${SHIPYARD_URL:-http://127.0.0.1:8080}"
COOKIE="$(mktemp)"
trap 'rm -f "$COOKIE"' EXIT

json_field() {
  python3 -c 'import json,sys; data=json.load(sys.stdin); path=sys.argv[1].split(".");
cur=data
for p in path: cur=cur[p]
print(cur)' "$1"
}

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
ORG_ID=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d "{\"slug\":\"smoke-org-$USER\",\"name\":\"Smoke Org\"}" \
  "$URL/api/v1/orgs" | json_field organization.id)
PROJ_ID=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d '{"slug":"smoke-app","name":"Smoke App"}' \
  "$URL/api/v1/orgs/$ORG_ID/projects" | json_field project.id)

PIPE_PAYLOAD=$(python3 - <<'PY'
import json
print(json.dumps({
  "slug": "smoke",
  "yaml": """pipeline:
  name: smoke
jobs:
  greet:
    runner:
      os: linux
    steps:
      - name: echo
        run: echo smoke
"""
}))
PY
)
PIPE_ID=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d "$PIPE_PAYLOAD" \
  "$URL/api/v1/orgs/$ORG_ID/projects/$PROJ_ID/pipelines" | json_field pipeline.id)

RUN=$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{}' \
  "$URL/api/v1/orgs/$ORG_ID/projects/$PROJ_ID/pipelines/$PIPE_ID/runs")
printf '%s\n' "$RUN" | grep -q '"status"'

echo "==> package repo"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' \
  -d '{"name":"libs","format":"maven"}' \
  "$URL/api/v1/orgs/$ORG_ID/projects/$PROJ_ID/packages" | grep -q maven

echo "==> oidc providers endpoint"
curl -fsS "$URL/api/v1/auth/oidc/providers" | grep -q providers

echo "==> oci api version"
curl -fsS "$URL/v2/" >/dev/null

echo "OK smoke against $URL"
