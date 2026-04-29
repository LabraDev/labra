#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$ROOT_DIR/labra-backend"
OUT_DIR="$BACKEND_DIR/dist"
OUT_FILE="$OUT_DIR/openapi.json"
LOG_FILE="/tmp/labra-openapi-server.log"
PID_FILE="/tmp/labra-openapi-server.pid"

cleanup() {
  if [[ -f "$PID_FILE" ]]; then
    local pid
    pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" 2>/dev/null || true
    fi
    rm -f "$PID_FILE"
  fi
}
trap cleanup EXIT

mkdir -p "$OUT_DIR"

(
  cd "$BACKEND_DIR"
  APP_ENV=dev \
  DB_URL="$OUT_DIR/openapi-ci.db" \
  JWT_ISSUER=labra-ci-issuer \
  JWT_AUDIENCE=labra-ci-audience \
  JWT_SIGNING_SECRET=labra-ci-signing-secret \
  GH_CLIENT_ID="" \
  GH_CLIENT_SECRET="" \
  GITHUB_WEBHOOK_SECRET=labra-ci-webhook-secret \
  nohup go run ./cmd >"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
)

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:8080/health" >/dev/null 2>&1; then
    break
  fi
  if [[ -f "$PID_FILE" ]]; then
    pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && ! kill -0 "$pid" >/dev/null 2>&1; then
      break
    fi
  fi
  sleep 1
done

if ! curl -fsS "http://127.0.0.1:8080/health" >/dev/null 2>&1; then
  echo "backend failed to start for OpenAPI generation" >&2
  tail -n 40 "$LOG_FILE" >&2 || true
  exit 1
fi

if curl -fsS "http://127.0.0.1:8080/swagger/openapi.json" >"$OUT_FILE" 2>/dev/null; then
  :
elif curl -fsS "http://127.0.0.1:8080/openapi.json" >"$OUT_FILE" 2>/dev/null; then
  :
else
  echo "failed to fetch OpenAPI document from backend" >&2
  tail -n 40 "$LOG_FILE" >&2 || true
  exit 1
fi

test -s "$OUT_FILE"
rm -f "$OUT_DIR/openapi-ci.db"
rm -rf "$BACKEND_DIR/doc"
echo "OpenAPI generated at $OUT_FILE"
