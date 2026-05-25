#!/usr/bin/env bash
# test_07_nack.sh — NACK on handler error (no infinite loop)
#   Proves that a malformed message triggers d.Nack(false, false):
#   message is dropped (not requeued), worker stays alive, valid
#   messages continue to be processed normally.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
API=http://localhost:8080
RMQ_API=http://localhost:15672

echo ""
echo "========================================="
echo "  TEST 07 — NACK on handler error"
echo "========================================="

pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

rmq_queue_depth() {
  curl -s -u guest:guest "$RMQ_API/api/queues/%2F/todo.notifications" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('messages_ready', 0))"
}

rmq_unacked() {
  curl -s -u guest:guest "$RMQ_API/api/queues/%2F/todo.notifications" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('messages_unacknowledged', 0))"
}

# ── step 1: ensure worker is running ─────────────────────────────────────────
echo ""
echo "--- Step 1: verify worker is running"
if ! pgrep -f "$BINARY worker" > /dev/null 2>&1; then
  : > "$WORKER_LOG"
  "$BINARY" worker >> "$WORKER_LOG" 2>&1 &
  for i in $(seq 1 20); do
    grep -q "rabbitmq notification consumer starting" "$WORKER_LOG" 2>/dev/null && break
    [ $i -eq 20 ] && fail "worker did not start within 20s"
    sleep 1
  done
fi
pass "worker is running"

NOTIF_BEFORE=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
ERROR_BEFORE=$( { grep 'consumer handler failed' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )

# ── step 2: inject malformed JSON via management API ────────────────────────
echo ""
echo "--- Step 2: inject malformed message (invalid JSON)"
curl -s -u guest:guest -X POST \
  "$RMQ_API/api/exchanges/%2F/todo.events/publish" \
  -H "Content-Type: application/json" \
  -d '{"properties":{"delivery_mode":2},"routing_key":"todo.created","payload":"not-valid-json","payload_encoding":"string"}' \
  > /dev/null
pass "malformed message injected"

# ── step 3: wait for worker to process (NACK) it ────────────────────────────
echo ""
echo "--- Step 3: wait for NACK (handler error log)"
for i in $(seq 1 10); do
  ERROR_AFTER=$( { grep 'consumer handler failed' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  if [ "$ERROR_AFTER" -gt "$ERROR_BEFORE" ]; then
    pass "handler error logged (NACK triggered)"; break
  fi
  [ $i -eq 10 ] && fail "no handler error logged within 10s"
  sleep 1
done

# ── step 4: message must not be requeued ─────────────────────────────────────
echo ""
echo "--- Step 4: verify message was dropped (not requeued)"
for i in $(seq 1 10); do
  DEPTH=$(rmq_queue_depth); DEPTH="${DEPTH//[$'\t\r\n ']}"
  UNACKED=$(rmq_unacked); UNACKED="${UNACKED//[$'\t\r\n ']}"
  if [ "$DEPTH" = "0" ] && [ "$UNACKED" = "0" ]; then
    pass "messages_ready=0 (not requeued)"
    pass "messages_unacknowledged=0"
    break
  fi
  [ $i -eq 10 ] && { [ "$DEPTH" != "0" ] && fail "messages_ready=$DEPTH, expected 0"; fail "messages_unacknowledged=$UNACKED, expected 0"; }
  sleep 1
done

# ── step 5: worker still alive ────────────────────────────────────────────────
echo ""
echo "--- Step 5: verify worker process still running"
pgrep -f "$BINARY worker" > /dev/null 2>&1 && pass "worker still running" || \
  fail "worker crashed after NACK"

# ── step 6: valid messages still processed ────────────────────────────────────
echo ""
echo "--- Step 6: valid notification still processed after NACK"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T07 post-nack valid"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE"

for i in $(seq 1 15); do
  NOTIF_AFTER=$( { grep '"notification task received"' "$WORKER_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  if [ "$NOTIF_AFTER" -gt "$NOTIF_BEFORE" ]; then
    pass "valid notification received after NACK (total: $NOTIF_AFTER)"; break
  fi
  [ $i -eq 15 ] && fail "valid notification not received within 15s after NACK"
  sleep 1
done

echo ""
echo "  TEST 07 PASSED ✅"
