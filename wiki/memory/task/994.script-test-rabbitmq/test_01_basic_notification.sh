#!/usr/bin/env bash
# test_01_basic_notification.sh — happy path: POST todo → notification delivered
#   Proves the end-to-end notification flow when everything is healthy:
#   POST todo → usecase writes outbox_event to DB → OutboxRelay → Kafka
#   → HandleDomainEvent → PublishNotification → RabbitMQ → worker ACKs → queue empty.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
API=http://localhost:8080
RMQ_API=http://localhost:15672

echo ""
echo "========================================="
echo "  TEST 01 — Basic notification (happy path)"
echo "========================================="

pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

rmq_queue_depth() {
  curl -s -u guest:guest "$RMQ_API/api/queues/%2F/todo.notifications" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('messages_ready', 0))"
}

# ── step 1: verify worker is running ─────────────────────────────────────────
echo ""
echo "--- Step 1: verify worker is running"
pgrep -f "$BINARY worker" > /dev/null 2>&1 && pass "worker is running" || \
  fail "worker is not running — run reset.sh first"

NOTIF_BEFORE=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )

# ── step 2: POST a todo ───────────────────────────────────────────────────────
echo ""
echo "--- Step 2: POST todo"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T01 happy path"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE"

# ── step 3: verify notification received by worker ───────────────────────────
echo ""
echo "--- Step 3: verify notification received"
for i in $(seq 1 10); do
  COUNT=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  if [ "$COUNT" -gt "$NOTIF_BEFORE" ]; then
    pass "notification received (total in log: $COUNT)"; break
  fi
  [ $i -eq 10 ] && fail "notification not received within 10s"
  sleep 1
done

# ── step 4: queue must be empty (message was ACKed) ───────────────────────────
echo ""
echo "--- Step 4: verify queue is empty after delivery"
for i in $(seq 1 10); do
  DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
  [ "$DEPTH" = "0" ] && pass "queue empty — message consumed and ACKed" && break
  [ $i -eq 10 ] && fail "queue still has $DEPTH message(s) after 10s"
  sleep 1
done

echo ""
echo "  TEST 01 PASSED ✅"
