#!/usr/bin/env bash
# test_08_competing_consumers.sh — two worker pods share the notification queue
#   POST 4 todos → total notifications across both worker pods = 4.
#   Assert both pods each received >= 1 message (RabbitMQ round-robin proved).
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

BATCH=4

echo ""
echo "========================================="
echo "  TEST 08 — Competing consumers"
echo "========================================="

# ── step 1: verify exactly 2 worker pods running ─────────────────────────────
echo ""
echo "--- Step 1: verify 2 worker pods running"
READY=$(kubectl -n "$NS" get deployment basesource-worker \
  -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
[ "$READY" -ge 2 ] && pass "$READY worker pods ready" || \
  fail "need >=2 worker pods, got $READY"

# get pod names for per-pod log inspection later
PODS=$(kubectl -n "$NS" get pod -l app=basesource-worker \
  -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
POD1=$(echo "$PODS" | awk '{print $1}')
POD2=$(echo "$PODS" | awk '{print $2}')
pass "pod1=$POD1"
pass "pod2=$POD2"

# snapshot per-pod notification counts before posting
NOTIF1_BEFORE=$(pod_logs "$POD1" | { grep -c '"notification task received"' || true; } | tr -d ' \t\r\n')
NOTIF2_BEFORE=$(pod_logs "$POD2" | { grep -c '"notification task received"' || true; } | tr -d ' \t\r\n')

# ── step 2: POST BATCH todos ──────────────────────────────────────────────────
echo ""
echo "--- Step 2: POST $BATCH todos"
for n in $(seq 1 $BATCH); do
  RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$API/api/v1/todos" \
    -H "Content-Type: application/json" -d "{\"title\":\"T08 K8s competing $n\"}")
  HTTP_CODE=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d: -f2)
  [ "$HTTP_CODE" = "201" ] && pass "todo $n created" || fail "todo $n: HTTP $HTTP_CODE"
done

# ── step 3: wait for all BATCH notifications across both pods ─────────────────
echo ""
echo "--- Step 3: wait for $BATCH total notifications across both pods"
for i in $(seq 1 20); do
  W1=$(pod_logs "$POD1" | { grep -c '"notification task received"' || true; } | tr -d ' \t\r\n')
  W2=$(pod_logs "$POD2" | { grep -c '"notification task received"' || true; } | tr -d ' \t\r\n')
  NEW1=$(( W1 - NOTIF1_BEFORE ))
  NEW2=$(( W2 - NOTIF2_BEFORE ))
  TOTAL=$(( NEW1 + NEW2 ))
  if [ "$TOTAL" -ge "$BATCH" ]; then
    pass "total notifications=$TOTAL (pod1 got $NEW1, pod2 got $NEW2)"; break
  fi
  [ $i -eq 20 ] && fail "only $TOTAL/$BATCH notifications after 20s (pod1=$NEW1, pod2=$NEW2)"
  sleep 1
done

# ── step 4: assert both pods received >= 1 message ───────────────────────────
echo ""
echo "--- Step 4: assert both pods each received >= 1 message (round-robin)"
W1_FINAL=$(pod_logs "$POD1" | { grep -c '"notification task received"' || true; } | tr -d ' \t\r\n')
W2_FINAL=$(pod_logs "$POD2" | { grep -c '"notification task received"' || true; } | tr -d ' \t\r\n')
NEW1_FINAL=$(( W1_FINAL - NOTIF1_BEFORE ))
NEW2_FINAL=$(( W2_FINAL - NOTIF2_BEFORE ))

echo "  pod1 ($POD1): $NEW1_FINAL notifications"
echo "  pod2 ($POD2): $NEW2_FINAL notifications"

[ "$NEW1_FINAL" -ge 1 ] && pass "pod1 received $NEW1_FINAL" || fail "pod1 received 0 messages — no round-robin"
[ "$NEW2_FINAL" -ge 1 ] && pass "pod2 received $NEW2_FINAL" || fail "pod2 received 0 messages — no round-robin"

echo ""
echo "  TEST 08 PASSED ✅"
