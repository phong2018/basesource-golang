#!/usr/bin/env bash
# reset.sh — kill processes, wipe DB, start API + worker ready for RabbitMQ tests
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
API_LOG=/tmp/api.log
WORKER_LOG=/tmp/worker.log
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"

info()  { echo "[reset] $*"; }
pass()  { echo "  ✅  $*"; }
fail()  { echo "  ❌  $*"; exit 1; }

wait_healthy() {
  local svc=$1 max=${2:-60} i=0
  while true; do
    ks=$($COMPOSE ps --format json "$svc" 2>/dev/null | \
         python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('Health',''))" 2>/dev/null || \
         $COMPOSE ps "$svc" 2>/dev/null | awk 'NR>1{print $NF}' | head -1)
    [[ "$ks" == *"healthy"* ]] && pass "$svc is healthy" && return
    [ $i -ge $max ] && fail "$svc did not become healthy after ${max}s"
    sleep 2; i=$((i+2))
  done
}

# ── 1. kill running processes ─────────────────────────────────────────────────
info "killing any running app processes..."
pkill -9 -f "basesource"    2>/dev/null || true
lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 6
pass "app processes killed"

# ── 2. TCP close wait ─────────────────────────────────────────────────────────
info "waiting for TCP connections to close..."
sleep 8
pass "connections closed"

# ── 3. bring up infrastructure ────────────────────────────────────────────────
info "starting docker-compose services..."
$COMPOSE up -d db rabbitmq kafka

info "waiting for services to be healthy..."
wait_healthy db 60
wait_healthy rabbitmq 60
wait_healthy kafka 90

# ── 4. truncate DB tables ─────────────────────────────────────────────────────
info "truncating DB tables..."
$COMPOSE exec -T db mysql -u appuser -papppass appdb -e \
  "SET FOREIGN_KEY_CHECKS=0; TRUNCATE TABLE outbox_deliveries; TRUNCATE TABLE outbox_events; TRUNCATE TABLE audit_logs; TRUNCATE TABLE todos; SET FOREIGN_KEY_CHECKS=1;" \
  2>/dev/null
pass "DB tables truncated"

# ── 5. purge RabbitMQ notification queue ─────────────────────────────────────
info "purging todo.notifications queue..."
curl -s -u guest:guest -X DELETE \
  "http://localhost:15672/api/queues/%2F/todo.notifications/contents" \
  -H "Content-Type: application/json" > /dev/null 2>&1 || true
pass "queue purged (or did not exist)"

# ── 6. build binary ───────────────────────────────────────────────────────────
info "building binary..."
cd "$REPO_ROOT"
go build -o "$BINARY" . 2>&1
pass "binary built: $BINARY"

# ── 7. run migrations ─────────────────────────────────────────────────────────
info "running migrations..."
"$BINARY" migrate 2>&1 | tail -3
pass "migrations done"

# ── 8. start API + worker ─────────────────────────────────────────────────────
info "starting API and worker..."
: > "$API_LOG"
: > "$WORKER_LOG"
"$BINARY" api    >> "$API_LOG"    2>&1 &
"$BINARY" worker >> "$WORKER_LOG" 2>&1 &

# ── 9. wait for API ready ─────────────────────────────────────────────────────
info "waiting for API to be ready..."
for i in $(seq 1 20); do
  curl -sf http://localhost:8080/health > /dev/null 2>&1 && \
    pass "API ready" && break
  [ $i -eq 20 ] && fail "API did not start within 20s — check $API_LOG"
  sleep 1
done

# ── 10. wait for worker ready ─────────────────────────────────────────────────
info "waiting for worker to be ready..."
for i in $(seq 1 20); do
  grep -q "rabbitmq notification consumer starting" "$WORKER_LOG" 2>/dev/null && \
    pass "worker ready" && break
  [ $i -eq 20 ] && fail "worker did not start within 20s — check $WORKER_LOG"
  sleep 1
done

echo ""
echo "========================================="
echo "  RESET COMPLETE — ready to run RabbitMQ tests"
echo "========================================="
