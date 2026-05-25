#!/usr/bin/env bash
# test_07_nack.sh — NACK on handler error (no infinite loop)
#   Inject malformed JSON via RabbitMQ management API → worker NACKs,
#   message is dropped (not requeued), worker pods stay alive,
#   valid messages continue to be processed normally.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

echo ""
echo "========================================="
echo "  TEST 07 — NACK on handler error"
echo "========================================="

# ── step 1: verify worker pods are running (2/2) ─────────────────────────────
echo ""
echo "--- Step 1: verify worker pods are running"
READY=$(kubectl -n "$NS" get deployment basesource-worker \
  -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
[ "$READY" -ge 2 ] && pass "worker pods ready: $READY/2" || \
  fail "worker not fully ready: $READY/2"

NOTIF_BEFORE=$(worker_log_count '"notification task received"')
ERROR_BEFORE=$(worker_log_count '"consumer handler failed, nacking"')

# ── step 2: inject malformed JSON via management API (exec inside pod) ───────
echo ""
echo "--- Step 2: inject malformed message (invalid JSON)"
rmq_publish_via_api "todo.events" "todo.created" "not-valid-json"
pass "malformed message injected"

# ── step 3: wait for NACK (handler error logged) ─────────────────────────────
echo ""
echo "--- Step 3: wait for NACK (handler error log)"
for i in $(seq 1 30); do
  ERROR_AFTER=$(worker_log_count '"consumer handler failed, nacking"')
  if [ "$ERROR_AFTER" -gt "$ERROR_BEFORE" ]; then
    pass "handler error logged — NACK triggered"; break
  fi
  [ $i -eq 30 ] && fail "no handler error logged within 30s"
  sleep 1
done

# ── step 4: message must not be requeued ──────────────────────────────────────
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
  [ $i -eq 10 ] && { [ "$DEPTH" != "0" ] && fail "messages_ready=$DEPTH, expected 0"; \
    fail "messages_unacknowledged=$UNACKED, expected 0"; }
  sleep 1
done

# ── step 5: worker pods still alive ───────────────────────────────────────────
echo ""
echo "--- Step 5: verify worker pods still running after NACK"
READY_AFTER=$(kubectl -n "$NS" get deployment basesource-worker \
  -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
[ "$READY_AFTER" -ge 2 ] && pass "worker pods still ready: $READY_AFTER/2" || \
  fail "worker pods crashed after NACK: $READY_AFTER/2"

# ── step 6: valid messages still processed after NACK ────────────────────────
echo ""
echo "--- Step 6: valid notification still processed after NACK"
RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" -d '{"title":"T07 K8s post-nack valid"}')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "HTTP 201" || fail "HTTP $HTTP_CODE"

for i in $(seq 1 30); do
  NOTIF_AFTER=$(worker_log_count '"notification task received"')
  if [ "$NOTIF_AFTER" -gt "$NOTIF_BEFORE" ]; then
    pass "valid notification received after NACK (total: $NOTIF_AFTER)"; break
  fi
  [ $i -eq 30 ] && fail "valid notification not received within 30s after NACK"
  sleep 1
done

echo ""
echo "  TEST 07 PASSED ✅"
