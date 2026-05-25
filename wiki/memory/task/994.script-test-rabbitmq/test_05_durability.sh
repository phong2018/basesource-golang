#!/usr/bin/env bash
# test_05_durability.sh — end-to-end durability via outbox pattern
#   Proves the correct flow: usecase writes outbox_deliveries to MySQL (not RabbitMQ).
#   When worker is offline the delivery stays pending in DB. On restart,
#   RabbitMQOutboxRelay picks up the pending delivery → publishes to RabbitMQ
#   todo.events exchange → NotificationConsumer handles it.
#
#   Flow: worker alive → POST todo → notification consumed ✓
#         kill worker → POST another todo → outbox_deliveries pending in DB, queue=0
#         restart worker → RabbitMQOutboxRelay → notification delivered ✓
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
API=http://localhost:8080
RMQ_API=http://localhost:15672
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"

echo ""
echo "========================================="
echo "  TEST 05 — Durability via outbox pattern"
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

# ── step 1: baseline — POST todo, worker alive, notification consumed ─────────
echo ""
echo "--- Step 1: baseline notification (worker alive)"
NOTIF_BEFORE=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )

RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T05 baseline"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE"

for i in $(seq 1 15); do
  COUNT=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  [ "$COUNT" -gt "$NOTIF_BEFORE" ] && pass "baseline notification received" && break
  [ $i -eq 15 ] && fail "baseline notification not received within 15s"
  sleep 1
done

# ── step 2: kill worker ───────────────────────────────────────────────────────
echo ""
echo "--- Step 2: kill worker"
kill_worker
pass "worker stopped (0 consumers on queue)"

# ── step 3: POST todo while worker is dead ────────────────────────────────────
echo ""
echo "--- Step 3: POST todo while worker is offline"
RESP2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T05 offline todo"}')
HTTP_CODE2=$(echo "$RESP2" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE2" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE2"

# ── step 4: outbox_event must be pending in DB (not in RabbitMQ) ─────────────
echo ""
echo "--- Step 4: verify outbox_event is pending in DB (worker offline)"
PENDING=$($COMPOSE exec -T db mysql -u appuser -papppass appdb \
  -e "SELECT COUNT(*) FROM outbox_deliveries WHERE destination='rabbitmq' AND status='pending';" 2>/dev/null \
  | tail -1 | tr -d ' \t\r\n')
[ "$PENDING" -ge 1 ] && pass "outbox_events pending=$PENDING in DB" || \
  fail "expected >=1 pending outbox_event, got $PENDING"

DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
[ "$DEPTH" = "0" ] && pass "RabbitMQ queue empty — usecase did not publish directly" || \
  fail "RabbitMQ queue=$DEPTH, expected 0 — usecase must not publish directly"

# ── step 5: restart worker ────────────────────────────────────────────────────
echo ""
echo "--- Step 5: restart worker"
: > "$WORKER_LOG"
"$BINARY" worker >> "$WORKER_LOG" 2>&1 &
for i in $(seq 1 20); do
  grep -q "rabbitmq notification consumer starting" "$WORKER_LOG" 2>/dev/null && \
    pass "worker restarted" && break
  [ $i -eq 20 ] && fail "worker did not restart within 20s"
  sleep 1
done

# ── step 6: notification delivered after relay → kafka → handler ─────────────
echo ""
echo "--- Step 6: verify notification delivered after restart"
for i in $(seq 1 20); do
  COUNT=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  [ "$COUNT" -ge 1 ] && pass "notification delivered (count in new log: $COUNT)" && break
  [ $i -eq 20 ] && fail "notification not delivered within 20s after restart"
  sleep 1
done

# ── step 7: outbox_event must now be published in DB ─────────────────────────
echo ""
echo "--- Step 7: verify outbox_event marked published in DB"
for i in $(seq 1 10); do
  PUBLISHED=$($COMPOSE exec -T db mysql -u appuser -papppass appdb \
    -e "SELECT COUNT(*) FROM outbox_deliveries WHERE destination='rabbitmq' AND status='published';" 2>/dev/null \
    | tail -1 | tr -d ' \t\r\n')
  [ "$PUBLISHED" -ge 1 ] && pass "outbox_events published=$PUBLISHED" && break
  [ $i -eq 10 ] && fail "outbox_event not marked published after 10s"
  sleep 1
done

# ── step 8: queue must be empty ───────────────────────────────────────────────
for i in $(seq 1 10); do
  DEPTH_AFTER=$(rmq_queue_depth); DEPTH_AFTER="${DEPTH_AFTER//[$'\t\r\n ']}"
  [ "$DEPTH_AFTER" = "0" ] && pass "queue empty after delivery" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH_AFTER message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 05 PASSED ✅"
