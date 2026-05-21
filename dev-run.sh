#!/usr/bin/env bash
# dev-run.sh — start mock vaults + syncer daemon for local development
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"

SOURCE_VAULT="http://localhost:8000"
TARGET_VAULT="http://localhost:8001"
DAEMON="http://localhost:8080"
SOURCE_TOKEN="source_admin_token_12345"
TARGET_TOKEN="target_admin_token_12345"

MOCK_PID=""
DAEMON_PID=""

cleanup() {
  echo ""
  echo "Stopping processes…"
  [[ -n "$DAEMON_PID" ]] && kill "$DAEMON_PID" 2>/dev/null || true
  [[ -n "$MOCK_PID"   ]] && kill "$MOCK_PID"   2>/dev/null || true
  wait 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT INT TERM

wait_for() {
  local url="$1" label="$2"
  echo -n "Waiting for $label… "
  for i in $(seq 1 30); do
    if curl -sf "$url" -o /dev/null 2>/dev/null; then
      echo "ready"
      return 0
    fi
    sleep 1
  done
  echo "TIMEOUT"
  return 1
}

# ── 0. Clean up any leftover processes from previous runs ────────────────────
echo "Cleaning up stale processes on ports 8000/8001/8080/9090…"
for port in 8000 8001 8080 9090; do
  fuser -k "${port}/tcp" 2>/dev/null || true
done
sleep 0.5

# ── 1. Build ─────────────────────────────────────────────────────────────────
echo "Building vaults-syncer…"
cd "$ROOT"
go build -o /tmp/vaults-syncer-dev . 2>&1

# ── 2. Mock vaults ────────────────────────────────────────────────────────────
echo "Starting mock vaults (source :8000, target :8001)…"
go run "$ROOT/testdata/mock-vault/main.go" \
  --source-port 8000 \
  --target-port 8001 \
  --source-token "$SOURCE_TOKEN" \
  --target-token "$TARGET_TOKEN" &
MOCK_PID=$!

wait_for "$SOURCE_VAULT/alive" "source vault"
wait_for "$TARGET_VAULT/alive" "target vault"

# ── 3. Seed source vault with test secrets ────────────────────────────────────
echo "Seeding source vault with test secrets…"
seed_secret() {
  local name="$1" value="$2"
  curl -sf -X POST "$SOURCE_VAULT/api/ciphers" \
    -H "Authorization: Bearer $SOURCE_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$name\",\"type\":1,\"login\":{\"username\":\"$name\",\"password\":\"$value\"}}" \
    -o /dev/null
  echo "  + $name"
}

seed_secret "db-password"       "s3cr3tDbPass!"
seed_secret "api-key-stripe"    "sk_test_abc123xyz"
seed_secret "jwt-signing-key"   "supersecretjwtsigningkey"
seed_secret "smtp-password"     "mailPass@2026"
seed_secret "redis-auth-token"  "redisSecure#Token"

# ── 4. Start syncer daemon ────────────────────────────────────────────────────
echo "Starting vaults-syncer daemon…"
/tmp/vaults-syncer-dev -config "$ROOT/config.local.yaml" -db /tmp/vaults-syncer-dev.db &
DAEMON_PID=$!

wait_for "$DAEMON/health" "syncer daemon"

# ── 5. Done ───────────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────────"
echo "  Web UI   →  http://localhost:8080/"
echo "  API      →  http://localhost:8080/syncs"
echo "  Health   →  http://localhost:8080/health"
echo "  Metrics  →  http://localhost:9090/metrics"
echo ""
echo "  Force sync:"
echo "  curl -X POST http://localhost:8080/syncs/local_sync/execute"
echo "────────────────────────────────────────────"
echo ""
echo "Press Ctrl+C to stop."
wait "$DAEMON_PID"
