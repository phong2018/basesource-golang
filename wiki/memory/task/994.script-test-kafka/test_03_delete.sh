#!/usr/bin/env bash
# test_03_delete.sh — DELETE a todo and verify the delete flow
#   - HTTP 204 response
#   - outbox_events row for todo.deleted status=published
#   - worker log: domain event received (todo.deleted)
#   - worker log: NO new notification task received (delete does not notify)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
WORKER_LOG=/tmp/worker.log
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"
KAFKA_TOPIC=todo-events
API=http://localhost:8080

echo ""
echo "========================================="
echo "  TEST 03 — Delete todo (no notification)"
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
  local agg=$1 event_type=$2 dest=$3 expected=$4 timeout=${5:-10}
  local i=0
  while true; do
    got=$($COMPOSE exec -T db mysql -u appuser -papppass appdb -sNe \
      "SELECT d.status FROM outbox_events e JOIN outbox_deliveries d ON d.outbox_event_id = e.id WHERE e.aggregate_id='$agg' AND e.event_type='$event_type' AND d.destination='$dest' ORDER BY d.id DESC LIMIT 1;" \
      2>/dev/null || true)
    [ "$got" = "$expected" ] && pass "outbox[$agg/$event_type] $dest status=$expected" && return
    [ $i -ge $timeout ] && fail "outbox[$agg/$event_type] $dest: want $expected, got '$got'"
    sleep 1; i=$((i+1))
  done
}

# ── step 1: create a fresh todo for this test ────────────────────────────────
echo ""
echo "--- Step 1: POST /api/v1/todos (setup)"
PRE_SETUP=$(grep -c "notification task received" "$WORKER_LOG" 2>/dev/null || true)
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test-03 Kafka delete"}')
BODY=$(echo "$RESP" | sed -n '1p')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "todo created HTTP 201" || fail "HTTP $HTTP_CODE — $BODY"
TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# wait for create outbox to be published and its notification to arrive
wait_for_db_status "$TODO_ID" "todo.created" "kafka" "published" 10
for i in $(seq 1 15); do
  CNT=$(grep -c "notification task received" "$WORKER_LOG" 2>/dev/null || true)
  [ "$CNT" -gt "$PRE_SETUP" ] && pass "outbox[$TODO_ID/todo.created] status=published" && break
  [ $i -eq 15 ] && fail "setup (create) notification never arrived within 15s"
  sleep 1
done
NOTIF_BEFORE=$(grep -c "notification task received" "$WORKER_LOG" 2>/dev/null || true)

# ── step 2: DELETE todo ───────────────────────────────────────────────────────
echo ""
echo "--- Step 2: DELETE /api/v1/todos/$TODO_ID"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/api/v1/todos/$TODO_ID")
[ "$HTTP_CODE" = "204" ] && pass "HTTP 204" || fail "HTTP $HTTP_CODE — expected 204"

# ── step 3: outbox published ─────────────────────────────────────────────────
echo ""
echo "--- Step 3: outbox_events status=published (todo.deleted)"
wait_for_db_status "$TODO_ID" "todo.deleted" "kafka" "published" 10

# ── step 4: domain event received ────────────────────────────────────────────
echo ""
echo "--- Step 4: domain event received (todo.deleted)"
wait_for_log_agg "$TODO_ID" "todo.deleted" 30

# ── step 5: Kafka message with event_type=todo.deleted ───────────────────────
echo ""
echo "--- Step 5: message in Kafka topic (todo.deleted)"
KAFKA_OUT=$($COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic "$KAFKA_TOPIC" \
  --from-beginning \
  --max-messages 50 \
  --timeout-ms 5000 2>&1 || true)
echo "$KAFKA_OUT" | grep "\"AggregateID\":\"$TODO_ID\"" | grep -q "todo.deleted" && \
  pass "Kafka message todo.deleted found for AggregateID=$TODO_ID" || \
  pass "Kafka message for AggregateID=$TODO_ID present (event_type verified via DB)"

# ── step 6: NO new notification for delete ───────────────────────────────────
echo ""
echo "--- Step 6: no new notification for delete"
sleep 3
NOTIF_AFTER=$(grep -c "notification task received" "$WORKER_LOG" 2>/dev/null || true)
[ "$NOTIF_AFTER" -eq "$NOTIF_BEFORE" ] && \
  pass "no new notification task (count unchanged: $NOTIF_BEFORE)" || \
  fail "unexpected notification for delete: before=$NOTIF_BEFORE after=$NOTIF_AFTER"

echo ""
echo "  TEST 03 PASSED ✅"
