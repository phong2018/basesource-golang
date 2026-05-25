#!/usr/bin/env bash
# test_01_create.sh — POST a todo and verify the full create flow
#   - HTTP 201 response
#   - outbox kafka delivery status=published
#   - outbox rabbitmq delivery status=published
#   - worker log: domain event received (todo.created)
#   - worker log: notification task received
#   - message present in Kafka topic
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

KAFKA_TOPIC=todo-events

echo ""
echo "========================================="
echo "  TEST 01 — Create todo (Kafka + RabbitMQ)"
echo "========================================="

# ── step 1: POST todo ────────────────────────────────────────────────────────
echo ""
echo "--- Step 1: POST /api/v1/todos"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" \
  -d '{"title":"T01 K8s create"}')
BODY=$(echo "$RESP" | sed -n '1p')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE — $BODY"

TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# ── step 2: kafka delivery published ─────────────────────────────────────────
echo ""
echo "--- Step 2: kafka delivery status=published"
wait_for_delivery "$TODO_ID" "todo.created" "kafka" "published" 15

# ── step 3: rabbitmq delivery published ──────────────────────────────────────
echo ""
echo "--- Step 3: rabbitmq delivery status=published"
wait_for_delivery "$TODO_ID" "todo.created" "rabbitmq" "published" 15

# ── step 4: domain event received in worker log ──────────────────────────────
echo ""
echo "--- Step 4: domain event received (todo.created)"
wait_for_worker_log_agg "$TODO_ID" "todo.created" 30

# ── step 5: notification task received ───────────────────────────────────────
echo ""
echo "--- Step 5: notification task received"
wait_for_worker_log "notification task received" 15

# ── step 6: message in Kafka topic ───────────────────────────────────────────
echo ""
echo "--- Step 6: message in Kafka topic $KAFKA_TOPIC"
KAFKA_OUT=$(kafka_exec /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic "$KAFKA_TOPIC" \
  --from-beginning \
  --max-messages 20 \
  --timeout-ms 5000 2>&1 || true)

echo "$KAFKA_OUT" | grep -q "\"AggregateID\":\"$TODO_ID\"" && \
  pass "Kafka message found for AggregateID=$TODO_ID" || \
  fail "Kafka message with AggregateID=$TODO_ID not found. Topic output: $KAFKA_OUT"

echo ""
echo "  TEST 01 PASSED ✅"
