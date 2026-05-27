#!/usr/bin/env bash
# test_01_basic_notification.sh — happy path: POST todo → notification delivered
#   Proves the end-to-end notification flow when everything is healthy:
#   POST todo → usecase writes outbox_deliveries to DB → RabbitMQOutboxRelay
#   → RabbitMQ todo.events exchange → NotificationConsumer ACKs → queue empty.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

echo ""
echo "========================================="
echo "  TEST 01 — Basic notification (happy path)"
echo "========================================="

# ── step 1: verify worker pods are running ───────────────────────────────────
echo ""
echo "--- Step 1: verify worker pods running"
READY=$(kubectl -n "$NS" get deployment basesource-worker \
  -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
[ "$READY" -ge 1 ] && pass "worker pods ready: $READY" || \
  fail "no worker pods ready — run reset.sh first"

# ── step 2: POST a todo ───────────────────────────────────────────────────────
echo ""
echo "--- Step 2: POST todo"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T01 K8s basic notification"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
BODY=$(echo "$RESP" | sed -n '1p')
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE — $BODY"

TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# ── step 3: rabbitmq delivery published ──────────────────────────────────────
echo ""
echo "--- Step 3: rabbitmq delivery status=published"
wait_for_delivery "$TODO_ID" "todo.created" "rabbitmq" "published" 15

# ── step 4: notification received by worker ───────────────────────────────────
echo ""
echo "--- Step 4: notification task received by worker"
wait_for_worker_log_agg "$TODO_ID" "todo.created" 30

# ── step 5: queue empty (message was ACKed) ───────────────────────────────────
echo ""
echo "--- Step 5: queue empty after delivery"
for i in $(seq 1 10); do
  DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
  [ "$DEPTH" = "0" ] && pass "queue empty — message consumed and ACKed" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 01 PASSED ✅"
