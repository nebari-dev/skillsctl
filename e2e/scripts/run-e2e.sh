#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"

E2E_TMPDIR=$(mktemp -d)
trap 'kill "$SERVER_PID" 2>/dev/null; rm -rf "$E2E_TMPDIR"' EXIT

echo "==> Building binaries..."
CGO_ENABLED=0 go build -o "$E2E_TMPDIR/skillsctl" ./cli
CGO_ENABLED=0 go build -o "$E2E_TMPDIR/skillsctl-server" ./backend/cmd/server

echo "==> Finding available port..."
PORT=18080
while nc -z localhost "$PORT" 2>/dev/null; do
    PORT=$((PORT + 1))
    if [ "$PORT" -gt 18180 ]; then
        echo "no available port found in range 18080-18180" >&2
        exit 1
    fi
done
echo "    Using port $PORT"

echo "==> Starting server (DEV_MODE)..."
DEV_MODE=true \
DB_PATH="$E2E_TMPDIR/e2e-test.db" \
PORT="$PORT" \
  "$E2E_TMPDIR/skillsctl-server" > "$E2E_TMPDIR/server.log" 2>&1 &
SERVER_PID=$!

DEADLINE=$((SECONDS + 10))
until curl -sf "http://localhost:$PORT/healthz" >/dev/null 2>&1; do
    if [ "$SECONDS" -ge "$DEADLINE" ]; then
        echo "server failed to start within 10s" >&2
        echo "--- server log ---"
        cat "$E2E_TMPDIR/server.log" >&2
        exit 1
    fi
    sleep 0.2
done
echo "    Server healthy"

echo "==> Running e2e tests..."
SKILLCTL_CLI_PATH="$E2E_TMPDIR/skillsctl" \
SKILLCTL_SERVER_URL="http://localhost:$PORT" \
  go test -tags e2e -timeout 120s -v ./e2e/...

echo "==> Done"
