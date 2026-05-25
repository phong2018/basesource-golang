#!/usr/bin/env bash
# test_06_queue_while_offline.sh — batch delivery after worker restart
#   Scale workers to 0 → POST 3 todos → outbox_deliveries pending, queue empty
#   → scale workers back to 2 → all notifications delivered, queue empty.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

BATCH=3

echo ""
echo "========================================="
echo "  TEST 06 — Batch delivery after worker restart"
echo "========================================="

# ── step 1: stop all workers ──────────────────────────────────────────────────
echo ""
echo "--- Step 1: scale workers to 0"
scale_deployment basesource-worker 0
pass "all worker pods stopped"

# confirm 0 consumers on queue
for i in $(seq 1 15); do
  C=$(rmq_consumers); C="${C//[$'\t\r\n ']}"
  [ "$C" = "0" ] && pass "RabbitMQ queue has 0 consumers" && break
  [ $i -eq 15 ] && fail "RabbitMQ still has consumers after scaling workers to 0"
  sleep 1
done

# ── step 2: POST BATCH todos while workers are down ───────────────────────────
echo ""
echo "--- Step 2: POST $BATCH todos while workers are offline"
for n in $(seq 1 $BATCH); do
  RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
    -H "Content-Type: application/json" -d "{\"title\":\"T06 K8s batch $n\"}")
  HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
  [ "$HTTP_CODE" = "201" ] && pass "todo $n created (HTTP 201)" || fail "todo $n: HTTP $HTTP_CODE"
done

# ── step 3: verify pending in DB, queue still empty ──────────────────────────
echo ""
echo "--- Step 3: verify $BATCH outbox_deliveries pending, RabbitMQ queue=0"
PENDING=$(db_exec "SELECT COUNT(*) FROM outbox_deliveries WHERE destination='rabbitmq' AND status='pending';" | tr -d ' \t\r\n')
[ "$PENDING" -ge "$BATCH" ] && pass "pending deliveries=$PENDING (expected >=$BATCH)" || \
  fail "expected >=$BATCH pending deliveries, got $PENDING"

DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
[ "$DEPTH" = "0" ] && pass "RabbitMQ queue=0 — usecase did not publish directly" || \
  fail "RabbitMQ queue=$DEPTH, expected 0 while workers are down"

# ── step 4: restart workers ───────────────────────────────────────────────────
echo ""
echo "--- Step 4: scale workers back to 2"
scale_deployment basesource-worker 2
pass "worker pods restarted (2/2 ready)"

# ── step 5: wait for all notifications ───────────────────────────────────────
echo ""
echo "--- Step 5: wait for $BATCH notifications to be delivered"
for i in $(seq 1 30); do
  COUNT=$(worker_log_count '"notification task received"')
  if [ "$COUNT" -ge "$BATCH" ]; then
    pass "received $COUNT notifications (expected $BATCH)"; break
  fi
  [ $i -eq 30 ] && fail "only $COUNT/$BATCH notifications received after 30s"
  sleep 1
done

# ── step 6: queue must be empty ───────────────────────────────────────────────
echo ""
echo "--- Step 6: queue empty after delivery"
for i in $(seq 1 10); do
  DEPTH_AFTER=$(rmq_queue_depth); DEPTH_AFTER="${DEPTH_AFTER//[$'\t\r\n ']}"
  [ "$DEPTH_AFTER" = "0" ] && pass "queue empty — all messages consumed" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH_AFTER message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 06 PASSED ✅"
