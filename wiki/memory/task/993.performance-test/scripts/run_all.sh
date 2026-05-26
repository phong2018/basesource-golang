#!/usr/bin/env bash
# run_all.sh — run the full performance test suite (smoke → load → stress)
#
# Usage:
#   bash run_all.sh                    # smoke + load (3 scenarios) + stress
#   bash run_all.sh --smoke-only       # sanity check only
#   bash run_all.sh --no-stress        # smoke + load, skip stress
#   bash run_all.sh --scenario=mixed   # smoke + one load scenario only
#
# Prerequisites:
#   brew install k6
#   kubectl port-forward will be started automatically.
#   metrics-server must be installed in kind (see Step 0 in the plan).
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
NS="basesource"
API="http://localhost:18080"
PF_PID=""

SMOKE_ONLY=0
NO_STRESS=0
SCENARIO="all"

for arg in "$@"; do
  case "$arg" in
    --smoke-only)    SMOKE_ONLY=1 ;;
    --no-stress)     NO_STRESS=1 ;;
    --scenario=*)    SCENARIO="${arg#*=}" ;;
  esac
done

# ── cleanup ───────────────────────────────────────────────────────────────────
cleanup() {
  [ -n "$PF_PID" ] && kill "$PF_PID" 2>/dev/null || true
}
trap cleanup EXIT

# ── header ────────────────────────────────────────────────────────────────────
START_TIME=$(date +%s)
echo ""
echo "########################################"
echo "  Performance Test Suite"
echo "########################################"
echo "  Namespace : $NS"
echo "  API       : $API"
echo "  Date      : $(date)"
echo "  Scenario  : $SCENARIO"
echo "########################################"

# ── preflight ─────────────────────────────────────────────────────────────────
command -v k6 >/dev/null 2>&1 || { echo "  ❌  k6 not found — run: brew install k6"; exit 1; }
kubectl -n "$NS" get pods > /dev/null 2>&1 || { echo "  ❌  cluster not reachable"; exit 1; }
echo "  ✅  cluster reachable"

# ── port-forward ──────────────────────────────────────────────────────────────
echo ""
echo ">>> Starting port-forward (API 18080→80)..."
pkill -f "kubectl.*port-forward.*18080" 2>/dev/null || true
sleep 1
kubectl -n "$NS" port-forward svc/basesource-api-svc 18080:80 > /dev/null 2>&1 &
PF_PID=$!

for i in $(seq 1 15); do
  curl -s "$API/health" > /dev/null 2>&1 && break
  [ $i -eq 15 ] && { echo "  ❌  API port-forward failed"; exit 1; }
  sleep 1
done
echo "  ✅  API port-forward live ($API)"

# ── smoke test ────────────────────────────────────────────────────────────────
echo ""
echo ">>> SMOKE TEST (1 VU, 30s)"
k6 run --vus 1 --duration 30s "$DIR/smoke_test.js"
echo "  ✅  smoke test passed"

[ $SMOKE_ONLY -eq 1 ] && { echo ""; echo "  Done (smoke only)."; exit 0; }

# ── load test ─────────────────────────────────────────────────────────────────
echo ""
if [ "$SCENARIO" = "all" ]; then
  echo ">>> LOAD TEST (all 3 scenarios — write_heavy → mixed → read_only)"
  k6 run "$DIR/load_test.js"
else
  echo ">>> LOAD TEST (scenario: $SCENARIO)"
  k6 run --env "SCENARIO=$SCENARIO" "$DIR/load_test.js"
fi
echo "  ✅  load test passed"

# ── stress test ───────────────────────────────────────────────────────────────
if [ $NO_STRESS -eq 0 ]; then
  echo ""
  echo ">>> STRESS TEST (find breaking point — no pass/fail threshold)"
  k6 run "$DIR/stress_test.js" || true  # stress test failing is expected
  echo "  ℹ️   stress test complete (check output above for breaking point)"
fi

# ── summary ───────────────────────────────────────────────────────────────────
END_TIME=$(date +%s)
echo ""
echo "########################################"
echo "  DONE"
echo "  Time : $((END_TIME - START_TIME))s"
echo "########################################"
echo ""
echo "  Next steps:"
echo "  1. Record results in 003.test-performance-plan.md (Step 5 table)"
echo "  2. Check HPA events:  kubectl describe hpa basesource-api-hpa -n $NS"
echo "  3. Check pod scaling: kubectl -n $NS get pods --watch (rerun during test)"
echo "  4. Analyze latency from logs:"
echo "     kubectl -n $NS logs -l app=basesource-api --tail=5000 | \\"
echo "       grep '\"msg\":\"request completed\"' | jq '.latency_ms' | sort -n | \\"
echo "       awk 'BEGIN{c=0;s=0} {c++;s+=\$1} END{print \"avg:\", s/c, \"ms  count:\", c}'"
