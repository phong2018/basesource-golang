#!/usr/bin/env bash
# test_01_create.sh — POST a todo and verify the full create flow
#   - HTTP 201 response
#   - outbox_events row status=published
#   - worker log: domain event received (todo.created)
#   - worker log: notification task received
#   - message present in Kafka topic
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
WORKER_LOG=/tmp/worker.log
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"
KAFKA_TOPIC=todo-events
API=http://localhost:8080

echo ""
echo "========================================="
echo "  TEST 01 — Create todo (Kafka + RabbitMQ)"
echo "========================================="

# ── helpers ───────────────────────────────────────────────────────────────────
pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

wait_for_log() {
  local pattern=$1 timeout=${2:-30}
  local i=0
  while ! grep -q "$pattern" "$WORKER_LOG" 2>/dev/null; do
    [ $i -ge $timeout ] && fail "timeout (${timeout}s) — pattern not found in worker log: $pattern"
    sleep 1; i=$((i+1))
  done
  pass "worker log: $pattern"
}

wait_for_log_agg() {
  local agg=$1 event_type=$2 timeout=${3:-30}
  local i=0
  while ! grep "\"aggregate_id\":\"$agg\"" "$WORKER_LOG" 2>/dev/null | grep -q "\"event_type\":\"$event_type\""; do
    [ $i -ge $timeout ] && fail "timeout (${timeout}s) — no $event_type for aggregate $agg in worker log"
    sleep 1; i=$((i+1))
  done
  pass "worker log: $event_type for aggregate $agg"
}

wait_for_db_status() {
  local agg=$1 event_type=$2 expected=$3 timeout=${4:-10}
  local i=0
  while true; do
    got=$($COMPOSE exec -T db mysql -u appuser -papppass appdb -sNe \
      "SELECT status FROM outbox_events WHERE aggregate_id='$agg' AND event_type='$event_type' ORDER BY id DESC LIMIT 1;" \
      2>/dev/null || true)
    [ "$got" = "$expected" ] && pass "outbox[$agg/$event_type] status=$expected" && return
    [ $i -ge $timeout ] && fail "outbox[$agg/$event_type]: want $expected, got '$got'"
    sleep 1; i=$((i+1))
  done
}

# ── step 1: POST todo ────────────────────────────────────────────────────────
echo ""
echo "--- Step 1: POST /api/v1/todos"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test-01 Kafka create"}')
BODY=$(echo "$RESP" | sed -n '1p')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)

[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE — expected 201. Body: $BODY"

TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# ── step 2: outbox published ─────────────────────────────────────────────────
echo ""
echo "--- Step 2: outbox_events status=published"
wait_for_db_status "$TODO_ID" "todo.created" "published" 10

# ── step 3: domain event received in worker log ──────────────────────────────
echo ""
echo "--- Step 3: domain event received (todo.created)"
wait_for_log_agg "$TODO_ID" "todo.created" 30

# ── step 4: notification task received ───────────────────────────────────────
echo ""
echo "--- Step 4: notification task received (RabbitMQ)"
wait_for_log "notification task received" 10

# ── step 5: message in Kafka topic ───────────────────────────────────────────
echo ""
echo "--- Step 5: message in Kafka topic $KAFKA_TOPIC"
KAFKA_OUT=$($COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
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
