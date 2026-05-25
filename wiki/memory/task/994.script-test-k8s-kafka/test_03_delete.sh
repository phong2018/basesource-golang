#!/usr/bin/env bash
# test_03_delete.sh — DELETE a todo and verify the delete flow
#   - HTTP 204 response
#   - outbox kafka + rabbitmq delivery status=published
#   - worker log: domain event received (todo.deleted)
#   - worker log: NO new notification task (delete does not notify)
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

echo ""
echo "========================================="
echo "  TEST 03 — Delete todo (no notification)"
echo "========================================="

# ── step 1: create a fresh todo ──────────────────────────────────────────────
echo ""
echo "--- Step 1: POST /api/v1/todos (setup)"
NOTIF_BEFORE=$(worker_log_count '"notification task received"')

RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
  -H "Content-Type: application/json" \
  -d '{"title":"T03 K8s delete"}')
BODY=$(echo "$RESP" | sed -n '1p')
HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
[ "$HTTP_CODE" = "201" ] && pass "todo created HTTP 201" || fail "HTTP $HTTP_CODE — $BODY"

TODO_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
[ -n "$TODO_ID" ] && pass "todo id=$TODO_ID" || fail "could not extract id from: $BODY"

# wait for create notification to drain
wait_for_delivery "$TODO_ID" "todo.created" "rabbitmq" "published" 15
for i in $(seq 1 15); do
  CNT=$(worker_log_count '"notification task received"')
  [ "$CNT" -gt "$NOTIF_BEFORE" ] && break
  [ $i -eq 15 ] && fail "setup (create) notification never arrived within 15s"
  sleep 1
done
NOTIF_BEFORE_DELETE=$(worker_log_count '"notification task received"')

# ── step 2: DELETE todo ───────────────────────────────────────────────────────
echo ""
echo "--- Step 2: DELETE /api/v1/todos/$TODO_ID"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/api/v1/todos/$TODO_ID")
[ "$HTTP_CODE" = "204" ] && pass "HTTP 204" || fail "HTTP $HTTP_CODE — expected 204"

# ── step 3: outbox published (kafka + rabbitmq) ───────────────────────────────
echo ""
echo "--- Step 3: kafka delivery status=published (todo.deleted)"
wait_for_delivery "$TODO_ID" "todo.deleted" "kafka" "published" 15

echo ""
echo "--- Step 4: rabbitmq delivery status=published (todo.deleted)"
wait_for_delivery "$TODO_ID" "todo.deleted" "rabbitmq" "published" 15

# ── step 5: domain event received ────────────────────────────────────────────
echo ""
echo "--- Step 5: domain event received (todo.deleted)"
wait_for_worker_log_agg "$TODO_ID" "todo.deleted" 30

# ── step 6: NO new notification for delete ───────────────────────────────────
echo ""
echo "--- Step 6: no new notification for delete"
sleep 3
NOTIF_AFTER=$(worker_log_count '"notification task received"')
[ "$NOTIF_AFTER" -eq "$NOTIF_BEFORE_DELETE" ] && \
  pass "no new notification (count unchanged: $NOTIF_BEFORE_DELETE)" || \
  fail "unexpected notification for delete: before=$NOTIF_BEFORE_DELETE after=$NOTIF_AFTER"

echo ""
echo "  TEST 03 PASSED ✅"
