#!/usr/bin/env bash
# test_06_queue_while_offline.sh — batch delivery after worker restart
#   Proves multiple outbox_events queued in DB while worker is down are all
#   relayed through Kafka → HandleDomainEvent → RabbitMQ when worker reconnects.
#
#   The usecase writes only to MySQL. Notifications are published by the worker's
#   domain event handler after the OutboxRelay sends events through Kafka.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
API=http://localhost:8080
RMQ_API=http://localhost:15672
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"
BATCH=3

echo ""
echo "========================================="
echo "  TEST 06 — Batch delivery after worker restart"
echo "========================================="

pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

rmq_queue_depth() {
  curl -s -u guest:guest "$RMQ_API/api/queues/%2F/todo.notifications" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('messages_ready', 0))"
}

rmq_consumers() {
  curl -s -u guest:guest "$RMQ_API/api/queues/%2F/todo.notifications" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('consumers', 0))"
}

kill_worker() {
  pkill -TERM -f "$BINARY worker" 2>/dev/null || true
  for i in $(seq 1 15); do
    pgrep -f "$BINARY worker" > /dev/null 2>&1 || break
    [ $i -eq 15 ] && { pkill -9 -f "$BINARY worker" 2>/dev/null || true; sleep 1; }
    sleep 1
  done
  for i in $(seq 1 15); do
    C=$(rmq_consumers); C="${C//[$'\t\r\n ']}"
    [ "$C" = "0" ] && return
    sleep 1
  done
}

# ── step 1: kill worker ───────────────────────────────────────────────────────
echo ""
echo "--- Step 1: kill worker"
kill_worker
pass "worker stopped (0 consumers on queue)"

# ── step 2: POST BATCH todos while worker is dead ─────────────────────────────
echo ""
echo "--- Step 2: POST $BATCH todos while worker is offline"
for n in $(seq 1 $BATCH); do
  RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
    -H "Content-Type: application/json" -d "{\"title\":\"T06 batch $n\"}")
  HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
  [ "$HTTP_CODE" = "201" ] && pass "todo $n created (HTTP 201)" || fail "todo $n: HTTP $HTTP_CODE"
done

# ── step 3: verify BATCH outbox_events pending in DB, queue still empty ───────
echo ""
echo "--- Step 3: verify $BATCH outbox_events pending in DB, RabbitMQ queue=0"
PENDING=$($COMPOSE exec -T db mysql -u appuser -papppass appdb \
  -e "SELECT COUNT(*) FROM outbox_deliveries WHERE destination='rabbitmq' AND status='pending';" 2>/dev/null \
  | tail -1 | tr -d ' \t\r\n')
[ "$PENDING" -ge "$BATCH" ] && pass "outbox_events pending=$PENDING (expected >=$BATCH)" || \
  fail "expected >=$BATCH pending outbox_events, got $PENDING"

DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
[ "$DEPTH" = "0" ] && pass "RabbitMQ queue=0 — usecase did not publish directly" || \
  fail "RabbitMQ queue=$DEPTH, expected 0"

# ── step 4: restart worker ────────────────────────────────────────────────────
echo ""
echo "--- Step 4: restart worker"
: > "$WORKER_LOG"
"$BINARY" worker >> "$WORKER_LOG" 2>&1 &
for i in $(seq 1 20); do
  grep -q "rabbitmq notification consumer starting" "$WORKER_LOG" 2>/dev/null && \
    pass "worker restarted" && break
  [ $i -eq 20 ] && fail "worker did not restart within 20s"
  sleep 1
done

# ── step 5: wait for all BATCH notifications to be delivered ─────────────────
echo ""
echo "--- Step 5: wait for all $BATCH notifications to be delivered"
for i in $(seq 1 30); do
  COUNT=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  if [ "$COUNT" -ge "$BATCH" ]; then
    pass "received $COUNT notifications (expected $BATCH)"; break
  fi
  [ $i -eq 30 ] && fail "only $COUNT/$BATCH notifications received after 30s"
  sleep 1
done

# ── step 6: queue must be empty ───────────────────────────────────────────────
for i in $(seq 1 10); do
  DEPTH_AFTER=$(rmq_queue_depth); DEPTH_AFTER="${DEPTH_AFTER//[$'\t\r\n ']}"
  [ "$DEPTH_AFTER" = "0" ] && pass "queue empty — all messages consumed" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH_AFTER message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 06 PASSED ✅"
