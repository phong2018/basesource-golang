#!/usr/bin/env bash
# test_05_independent_relay.sh — relay independence test
#   Proves that KafkaOutboxRelay and RabbitMQOutboxRelay are fully independent:
#   stop RabbitMQ → POST todo → kafka delivery becomes published while
#   rabbitmq delivery stays pending → restart RabbitMQ → rabbitmq delivery catches up.
#
#   This is the key dual-relay guarantee: one broker's failure has zero impact
#   on the other broker's delivery.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
WORKER_LOG=/tmp/worker.log
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"
API=http://localhost:8080

echo ""
echo "========================================="
echo "  TEST 05 — Independent relay failure"
echo "========================================="

pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

db_delivery_status() {
  local agg=$1 event_type=$2 dest=$3
  $COMPOSE exec -T db mysql -u appuser -papppass appdb -sNe \
    "SELECT d.status FROM outbox_events e JOIN outbox_deliveries d ON d.outbox_event_id = e.id WHERE e.aggregate_id='$agg' AND e.event_type='$event_type' AND d.destination='$dest' ORDER BY d.id DESC LIMIT 1;" \
    2>/dev/null | tr -d ' \t\r\n' || true
}

wait_for_delivery() {
  local agg=$1 event_type=$2 dest=$3 expected=$4 timeout=${5:-15}
  local i=0
  while true; do
    got=$(db_delivery_status "$agg" "$event_type" "$dest")
    [ "$got" = "$expected" ] && pass "outbox[$agg/$event_type] $dest=$expected" && return
    [ $i -ge $timeout ] && fail "outbox[$agg/$event_type] $dest: want $expected, got '$got'"
    sleep 1; i=$((i+1))
  done
}

# ── step 1: stop RabbitMQ ─────────────────────────────────────────────────────
echo ""
echo "--- Step 1: stop RabbitMQ"
$COMPOSE stop rabbitmq
pass "RabbitMQ stopped"

# ── step 2: POST todo ─────────────────────────────────────────────────────────
echo ""
echo "--- Step 2: POST todo while RabbitMQ is down"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" \
  -d '{"title":"T05 independent relay"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
BODY=$(echo "$RESP" | sed -n '1p')
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201 — API unaffected by RabbitMQ outage" || \
  fail "HTTP $HTTP_CODE — $BODY"

TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# ── step 3: kafka delivery must become published ──────────────────────────────
echo ""
echo "--- Step 3: kafka delivery must become published (relay unaffected by RabbitMQ)"
wait_for_delivery "$TODO_ID" "todo.created" "kafka" "published" 15

# ── step 4: rabbitmq delivery must stay pending ───────────────────────────────
echo ""
echo "--- Step 4: rabbitmq delivery must stay pending (broker down)"
sleep 3
RMQ_STATUS=$(db_delivery_status "$TODO_ID" "todo.created" "rabbitmq")
[ "$RMQ_STATUS" = "pending" ] || [ "$RMQ_STATUS" = "failed" ] && \
  pass "rabbitmq delivery=$RMQ_STATUS (not published while broker is down)" || \
  fail "rabbitmq delivery=$RMQ_STATUS — expected pending or failed"

# ── step 5: restart RabbitMQ ──────────────────────────────────────────────────
echo ""
echo "--- Step 5: restart RabbitMQ"
$COMPOSE start rabbitmq

# wait for RabbitMQ to be healthy
for i in $(seq 1 30); do
  STATUS=$($COMPOSE ps rabbitmq --format json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0].get('Health','') if isinstance(d,list) else d.get('Health',''))" 2>/dev/null || true)
  curl -s -u guest:guest http://localhost:15672/api/overview > /dev/null 2>&1 && \
    pass "RabbitMQ healthy" && break
  [ $i -eq 30 ] && fail "RabbitMQ did not become healthy within 30s"
  sleep 1
done

# ── step 6: rabbitmq delivery must catch up ───────────────────────────────────
echo ""
echo "--- Step 6: rabbitmq delivery catches up after broker restart"
wait_for_delivery "$TODO_ID" "todo.created" "rabbitmq" "published" 20

# ── step 7: notification task received ────────────────────────────────────────
echo ""
echo "--- Step 7: notification task received after catch-up"
for i in $(seq 1 15); do
  grep -q "notification task received" "$WORKER_LOG" 2>/dev/null && \
    pass "notification task received" && break
  [ $i -eq 15 ] && fail "notification task not received within 15s"
  sleep 1
done

echo ""
echo "  TEST 05 PASSED ✅"
