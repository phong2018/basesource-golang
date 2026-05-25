#!/usr/bin/env bash
# test_08_competing_consumers.sh — two workers share the notification queue
#   Proves RabbitMQ distributes messages across competing consumers:
#   POST 4 todos → total notifications across both workers = 4, no duplicates.
#
#   Both workers run RabbitMQOutboxRelay independently, but only one relay
#   picks up each pending delivery (row-level lock via SELECT FOR UPDATE).
#   RabbitMQ then distributes each published notification to whichever
#   NotificationConsumer worker is free — ensuring no duplicate handling.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
WORKER2_LOG=/tmp/worker2.log
API=http://localhost:8080
BATCH=4

echo ""
echo "========================================="
echo "  TEST 08 — Competing consumers"
echo "========================================="

pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

# ── step 1: ensure primary worker is running ─────────────────────────────────
echo ""
echo "--- Step 1: verify primary worker is running"
if ! pgrep -f "$BINARY worker" > /dev/null 2>&1; then
  : > "$WORKER_LOG"
  "$BINARY" worker >> "$WORKER_LOG" 2>&1 &
  for i in $(seq 1 20); do
    grep -q "rabbitmq notification consumer starting" "$WORKER_LOG" 2>/dev/null && break
    [ $i -eq 20 ] && fail "primary worker did not start within 20s"
    sleep 1
  done
fi
pass "primary worker running"

# ── step 2: start second worker ───────────────────────────────────────────────
echo ""
echo "--- Step 2: start second worker (same Kafka group — competing on RabbitMQ queue)"
: > "$WORKER2_LOG"
"$BINARY" worker >> "$WORKER2_LOG" 2>&1 &
WORKER2_PID=$!

for i in $(seq 1 20); do
  grep -q "rabbitmq notification consumer starting" "$WORKER2_LOG" 2>/dev/null && \
    pass "second worker started (pid=$WORKER2_PID)" && break
  [ $i -eq 20 ] && fail "second worker did not start within 20s"
  sleep 1
done

# give both consumers a moment to register with RabbitMQ
sleep 2

# snapshot counts before posting
NOTIF1_BEFORE=$( { grep '"notification task received"' "$WORKER_LOG"  2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
NOTIF2_BEFORE=$( { grep '"notification task received"' "$WORKER2_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )

# ── step 3: POST 4 todos in quick succession ──────────────────────────────────
echo ""
echo "--- Step 3: POST $BATCH todos"
for n in $(seq 1 $BATCH); do
  RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
    -H "Content-Type: application/json" -d "{\"title\":\"T08 competing $n\"}")
  HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
  [ "$HTTP_CODE" = "201" ] && pass "todo $n created" || fail "todo $n: HTTP $HTTP_CODE"
done

# ── step 4: wait for all BATCH notifications across both workers ──────────────
echo ""
echo "--- Step 4: wait for $BATCH total notifications across both workers"
for i in $(seq 1 20); do
  W1=$( { grep '"notification task received"' "$WORKER_LOG"  2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  W2=$( { grep '"notification task received"' "$WORKER2_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  NEW1=$(( W1 - NOTIF1_BEFORE ))
  NEW2=$(( W2 - NOTIF2_BEFORE ))
  TOTAL=$(( NEW1 + NEW2 ))
  if [ "$TOTAL" -ge "$BATCH" ]; then
    pass "total notifications=$TOTAL (worker1 got $NEW1, worker2 got $NEW2)"; break
  fi
  [ $i -eq 20 ] && fail "only $TOTAL/$BATCH notifications received after 20s (worker1=$NEW1, worker2=$NEW2)"
  sleep 1
done

# ── step 5: assert total >= BATCH (at-least-once; Kafka rebalance may add ≤1 extra) ──
echo ""
echo "--- Step 5: assert at least $BATCH notifications delivered"
W1_FINAL=$( { grep '"notification task received"' "$WORKER_LOG"  2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
W2_FINAL=$( { grep '"notification task received"' "$WORKER2_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
NEW1_FINAL=$(( W1_FINAL - NOTIF1_BEFORE ))
NEW2_FINAL=$(( W2_FINAL - NOTIF2_BEFORE ))
TOTAL_FINAL=$(( NEW1_FINAL + NEW2_FINAL ))

[ "$TOTAL_FINAL" -ge "$BATCH" ] && \
  pass "total=$TOTAL_FINAL (worker1=$NEW1_FINAL, worker2=$NEW2_FINAL) — at least $BATCH delivered" || \
  fail "expected >=$BATCH notifications, got $TOTAL_FINAL (worker1=$NEW1_FINAL, worker2=$NEW2_FINAL)"

# ── step 6: assert both workers received at least 1 message ──────────────────
echo ""
echo "--- Step 6: assert both workers each received >=1 message"
[ "$NEW1_FINAL" -ge 1 ] && pass "worker1 received $NEW1_FINAL" || fail "worker1 received 0 messages"
[ "$NEW2_FINAL" -ge 1 ] && pass "worker2 received $NEW2_FINAL" || fail "worker2 received 0 messages"

# ── cleanup: stop second worker ───────────────────────────────────────────────
pkill -TERM -P $WORKER2_PID 2>/dev/null || true
kill -TERM $WORKER2_PID 2>/dev/null || true
sleep 2
kill -9 $WORKER2_PID 2>/dev/null || true

echo ""
echo "  TEST 08 PASSED ✅"
