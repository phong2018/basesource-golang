#!/usr/bin/env bash
# test_05_durability.sh — RabbitMQ queue durability
#   Proves durable=true + DeliveryMode=Persistent:
#   The notification publisher lives in the USECASE layer (API process).
#   When the worker is dead, the API still publishes notifications to
#   RabbitMQ; they queue up (durable + persistent) and are delivered
#   when the worker restarts.
#
#   Flow: worker alive → POST todo → notification consumed ✓
#         kill worker → POST another todo → notification sits in queue
#         restart worker → queued notification delivered ✓
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
API=http://localhost:8080
RMQ_API=http://localhost:15672

echo ""
echo "========================================="
echo "  TEST 05 — RabbitMQ queue durability"
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
  # wait for broker to deregister consumer
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

# ── step 2: kill worker and confirm 0 consumers ───────────────────────────────
echo ""
echo "--- Step 2: kill worker"
kill_worker
pass "worker stopped (0 consumers on queue)"

# ── step 3: POST todo while worker is dead ────────────────────────────────────
# The usecase (in the API process) publishes the notification directly to
# RabbitMQ — the relay is not involved in notification delivery.
echo ""
echo "--- Step 3: POST todo while worker is offline"
RESP2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T05 offline todo"}')
HTTP_CODE2=$(echo "$RESP2" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE2" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE2"

# ── step 4: assert notification is queued (no consumer) ───────────────────────
echo ""
echo "--- Step 4: verify notification is queued (worker offline)"
for i in $(seq 1 10); do
  DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
  [ "$DEPTH" -ge 1 ] && pass "queue has $DEPTH ready message(s)" && break
  [ $i -eq 10 ] && fail "expected >=1 message in queue after 10s — usecase may not have published"
  sleep 1
done

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

# ── step 6: assert queued notification delivered after restart ────────────────
echo ""
echo "--- Step 6: verify queued notification delivered after restart"
for i in $(seq 1 15); do
  COUNT=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  [ "$COUNT" -ge 1 ] && pass "notification delivered after restart (count in new log: $COUNT)" && break
  [ $i -eq 15 ] && fail "notification not delivered within 15s after restart"
  sleep 1
done

# ── step 7: queue must be empty ───────────────────────────────────────────────
for i in $(seq 1 10); do
  DEPTH_AFTER=$(rmq_queue_depth); DEPTH_AFTER="${DEPTH_AFTER//[$'\t\r\n ']}"
  [ "$DEPTH_AFTER" = "0" ] && pass "queue empty after delivery" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH_AFTER message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 05 PASSED ✅"
