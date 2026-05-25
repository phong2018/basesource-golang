#!/usr/bin/env bash
# test_05_durability.sh — end-to-end durability via outbox pattern
#   Proves the correct flow: usecase writes outbox_deliveries to MySQL (not RabbitMQ).
#   When workers are offline the delivery stays pending in DB. On restart,
#   RabbitMQOutboxRelay picks up the pending delivery → publishes to RabbitMQ
#   todo.events exchange → NotificationConsumer handles it.
#
#   Flow: worker alive → POST todo → notification consumed ✓
#         scale workers to 0 → POST another todo → pending in DB, queue=0
#         scale workers to 2 → RabbitMQOutboxRelay → notification delivered ✓
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

echo ""
echo "========================================="
echo "  TEST 05 — Durability via outbox pattern"
echo "========================================="

# ── step 1: baseline — POST todo, workers alive, notification consumed ────────
echo ""
echo "--- Step 1: baseline notification (workers alive)"
NOTIF_BEFORE=$(worker_log_count '"notification task received"')

RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T05 K8s baseline"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE"
TODO_BASELINE=$(echo "$RESP" | sed -n '1p' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)

wait_for_delivery "$TODO_BASELINE" "todo.created" "rabbitmq" "published" 15
for i in $(seq 1 30); do
  COUNT=$(worker_log_count '"notification task received"')
  [ "$COUNT" -gt "$NOTIF_BEFORE" ] && pass "baseline notification received" && break
  [ $i -eq 30 ] && fail "baseline notification not received within 30s"
  sleep 1
done

# ── step 2: scale workers to 0 ───────────────────────────────────────────────
echo ""
echo "--- Step 2: scale workers to 0"
scale_deployment basesource-worker 0
pass "all worker pods stopped"
for i in $(seq 1 20); do
  C=$(rmq_consumers); C="${C//[$'\t\r\n ']}"
  [ "$C" = "0" ] && pass "RabbitMQ queue has 0 consumers" && break
  [ $i -eq 20 ] && fail "consumers still present after scaling workers to 0"
  sleep 1
done

# ── step 3: POST todo while workers are offline ───────────────────────────────
echo ""
echo "--- Step 3: POST todo while workers are offline"
RESP2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T05 K8s offline"}')
HTTP_CODE2=$(echo "$RESP2" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE2" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE2"
TODO_OFFLINE=$(echo "$RESP2" | sed -n '1p' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)

# ── step 4: verify pending in DB, queue still empty ──────────────────────────
echo ""
echo "--- Step 4: verify outbox_delivery pending in DB, RabbitMQ queue=0"
PENDING=$(db_exec "SELECT COUNT(*) FROM outbox_deliveries WHERE destination='rabbitmq' AND status='pending';" | tr -d ' \t\r\n')
[ "$PENDING" -ge 1 ] && pass "pending deliveries=$PENDING in DB" || \
  fail "expected >=1 pending delivery, got $PENDING"

DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
[ "$DEPTH" = "0" ] && pass "RabbitMQ queue=0 — usecase did not publish directly" || \
  fail "RabbitMQ queue=$DEPTH, expected 0"

# ── step 5: scale workers back to 2 ──────────────────────────────────────────
echo ""
echo "--- Step 5: scale workers back to 2"
scale_deployment basesource-worker 2
pass "worker pods restarted (2/2 ready)"

# ── step 6: notification delivered after relay picks up pending delivery ──────
echo ""
echo "--- Step 6: notification delivered after relay catches up"
wait_for_worker_log "notification task received" 30

# ── step 7: outbox_delivery must now be published ────────────────────────────
echo ""
echo "--- Step 7: outbox_delivery marked published in DB"
wait_for_delivery "$TODO_OFFLINE" "todo.created" "rabbitmq" "published" 15

# ── step 8: queue empty ───────────────────────────────────────────────────────
echo ""
echo "--- Step 8: queue empty after delivery"
for i in $(seq 1 10); do
  DEPTH_AFTER=$(rmq_queue_depth); DEPTH_AFTER="${DEPTH_AFTER//[$'\t\r\n ']}"
  [ "$DEPTH_AFTER" = "0" ] && pass "queue empty" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH_AFTER message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 05 PASSED ✅"
