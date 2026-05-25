#!/usr/bin/env bash
# test_05_independent_relay.sh — relay independence test
#   Stop RabbitMQ pod → POST todo → kafka delivery published, rabbitmq stays pending
#   → restart RabbitMQ pod → rabbitmq delivery catches up.
#   Proves dual-relay guarantee: one broker failure has zero impact on the other.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

echo ""
echo "========================================="
echo "  TEST 05 — Independent relay failure"
echo "========================================="

# ── step 1: stop RabbitMQ pod ────────────────────────────────────────────────
echo ""
echo "--- Step 1: stop RabbitMQ (scale to 0)"
scale_deployment rabbitmq 0
pass "RabbitMQ pod stopped"

# ── step 2: POST todo while RabbitMQ is down ─────────────────────────────────
echo ""
echo "--- Step 2: POST todo while RabbitMQ is down"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" \
  -d '{"title":"T05 K8s independent relay"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
BODY=$(echo "$RESP" | sed -n '1p')
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201 — API unaffected by RabbitMQ outage" || \
  fail "HTTP $HTTP_CODE — $BODY"

TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# ── step 3: kafka delivery must become published ──────────────────────────────
echo ""
echo "--- Step 3: kafka delivery published (relay unaffected by RabbitMQ outage)"
wait_for_delivery "$TODO_ID" "todo.created" "kafka" "published" 15

# ── step 4: rabbitmq delivery must stay pending ───────────────────────────────
echo ""
echo "--- Step 4: rabbitmq delivery stays pending (broker is down)"
sleep 5
RMQ_STATUS=$(db_delivery_status "$TODO_ID" "todo.created" "rabbitmq")
[ "$RMQ_STATUS" = "pending" ] || [ "$RMQ_STATUS" = "failed" ] && \
  pass "rabbitmq delivery=$RMQ_STATUS (not published while broker is down)" || \
  fail "rabbitmq delivery=$RMQ_STATUS — expected pending or failed"

# ── step 5: restart RabbitMQ pod ─────────────────────────────────────────────
echo ""
echo "--- Step 5: restart RabbitMQ (scale to 1)"
scale_deployment rabbitmq 1
pass "RabbitMQ pod restarted"

wait_for_rmq_ready 60

# ── step 6: rabbitmq delivery catches up ──────────────────────────────────────
echo ""
echo "--- Step 6: rabbitmq delivery catches up after broker restart"
wait_for_delivery "$TODO_ID" "todo.created" "rabbitmq" "published" 30

# ── step 7: notification task received ────────────────────────────────────────
echo ""
echo "--- Step 7: notification task received after catch-up"
wait_for_worker_log "notification task received" 15

echo ""
echo "  TEST 05 PASSED ✅"
