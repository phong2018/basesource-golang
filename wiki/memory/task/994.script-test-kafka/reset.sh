#!/usr/bin/env bash
# reset.sh — wipe DB data, reset Kafka offsets, rebuild binary, start API+worker
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
API_LOG=/tmp/api.log
WORKER_LOG=/tmp/worker.log
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"
KAFKA_GROUP=todo-worker
KAFKA_TOPIC=todo-events

# ── colour helpers ────────────────────────────────────────────────────────────
info()  { echo "[reset] $*"; }
pass()  { echo "  ✅  $*"; }
fail()  { echo "  ❌  $*"; exit 1; }

wait_healthy() {
  local svc=$1 max=${2:-60} i=0
  while true; do
    ks=$($COMPOSE ps --format json "$svc" 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('Health',''))" 2>/dev/null || \
         $COMPOSE ps "$svc" 2>/dev/null | awk 'NR>1{print $NF}' | head -1)
    [[ "$ks" == *"healthy"* ]] && pass "$svc is healthy" && return
    [ $i -ge $max ] && fail "$svc did not become healthy after ${max}s"
    sleep 2; i=$((i+2))
  done
}

# ── 1. kill running processes ─────────────────────────────────────────────────
info "killing any running app processes (binary + go-build cache)..."
pkill -9 -f "basesource"       2>/dev/null || true
pkill -9 -f "go-build.*api"    2>/dev/null || true
pkill -9 -f "go-build.*worker" 2>/dev/null || true
pkill -9 -f "go-build.*main"   2>/dev/null || true
# kill anything holding :8080
lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true
# kill any remaining go-build cache binaries (macOS: match by cmdline path)
pgrep -f "go-build" 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 6
pass "app processes killed"

# ── 2. brief pause for TCP connections to close ──────────────────────────────
info "waiting for app TCP connections to close..."
sleep 8
pass "connections closed"

# ── 3. bring up infrastructure ───────────────────────────────────────────────
info "starting docker-compose services..."
$COMPOSE up -d db rabbitmq kafka

info "waiting for services to be healthy..."
wait_healthy db 60
wait_healthy rabbitmq 60
wait_healthy kafka 90

# ── 4. truncate DB tables ────────────────────────────────────────────────────
info "truncating DB tables..."
$COMPOSE exec -T db mysql -u appuser -papppass appdb -e \
  "SET FOREIGN_KEY_CHECKS=0; TRUNCATE TABLE outbox_events; TRUNCATE TABLE audit_logs; TRUNCATE TABLE todos; SET FOREIGN_KEY_CHECKS=1;" \
  2>/dev/null
pass "DB tables truncated"

# ── 5a. delete consumer group to clear stale sessions ────────────────────────
info "deleting consumer group '$KAFKA_GROUP' to clear stale sessions..."
$COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group "$KAFKA_GROUP" --delete 2>/dev/null || true
pass "consumer group deleted (or did not exist)"

# ── 5b. delete + recreate Kafka topic (wipes all old messages) ───────────────
info "deleting Kafka topic '$KAFKA_TOPIC' to clear old messages..."
$COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --delete --topic "$KAFKA_TOPIC" 2>/dev/null || true

# wait for deletion to propagate
for i in $(seq 1 10); do
  exists=$($COMPOSE exec -T kafka \
    /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server localhost:9092 \
    --list 2>/dev/null | grep -c "^${KAFKA_TOPIC}$" || true)
  [ "$exists" = "0" ] && break
  sleep 1
done
pass "Kafka topic deleted"

info "recreating Kafka topic '$KAFKA_TOPIC' with 1 partition..."
$COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --topic "$KAFKA_TOPIC" \
  --partitions 1 --replication-factor 1 2>/dev/null || true
pass "Kafka topic recreated"

# ── 6. build binary ───────────────────────────────────────────────────────────
info "building binary..."
cd "$REPO_ROOT"
go build -o "$BINARY" . 2>&1
pass "binary built: $BINARY"

# ── 7. run migrations ─────────────────────────────────────────────────────────
info "running migrations..."
"$BINARY" migrate 2>&1 | tail -3
pass "migrations done"

# ── 8. start API + worker ────────────────────────────────────────────────────
info "starting API and worker..."
: > "$API_LOG"
: > "$WORKER_LOG"
"$BINARY" api  >> "$API_LOG"  2>&1 &
"$BINARY" worker >> "$WORKER_LOG" 2>&1 &

# ── 9. wait for worker ready + consumer group stable ─────────────────────────
info "waiting for worker to be ready..."
for i in $(seq 1 15); do
  grep -q "outbox relay starting" "$WORKER_LOG" 2>/dev/null && \
    pass "worker process started" && break
  [ $i -eq 15 ] && fail "worker did not start within 15s — check $WORKER_LOG"
  sleep 1
done

info "waiting for consumer group to stabilize (exactly 1 active member)..."
for i in $(seq 1 40); do
  # Count data rows (skip header line that contains "CONSUMER-ID" literally)
  member_count=$($COMPOSE exec -T kafka \
    /opt/kafka/bin/kafka-consumer-groups.sh \
    --bootstrap-server localhost:9092 \
    --group "$KAFKA_GROUP" --describe 2>/dev/null | \
    grep -v "^GROUP\|^$\|CONSUMER-ID" | grep -c "." || true)
  member_count="${member_count//[$'\t\r\n ']}"
  if [ "$member_count" = "1" ]; then
    pass "consumer group stable (1 member)"; break
  fi
  [ $i -eq 40 ] && { echo "  ⚠️  consumer group not stable after 80s (members=$member_count), proceeding anyway"; break; }
  sleep 2
done
pass "worker ready"

echo ""
echo "========================================="
echo "  RESET COMPLETE — ready to run tests"
echo "========================================="
