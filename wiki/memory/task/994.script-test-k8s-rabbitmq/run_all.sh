#!/usr/bin/env bash
# run_all.sh — RabbitMQ K8s integration test suite
# Usage:
#   bash run_all.sh            # full suite (reset + all tests)
#   bash run_all.sh --no-reset # skip reset
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

START_TIME=$(date +%s)
SKIP_RESET=0
for arg in "$@"; do [ "$arg" = "--no-reset" ] && SKIP_RESET=1; done

echo ""
echo "########################################"
echo "  RabbitMQ K8s Integration Test Suite"
echo "########################################"
echo "  Namespace  : $NS"
echo "  API        : $API"
echo "  Date       : $(date)"
echo "########################################"

kubectl -n "$NS" get pods > /dev/null 2>&1 || {
  echo "  ❌  cluster not reachable"
  exit 1
}
echo "  ✅  cluster reachable"

echo ""
echo ">>> Starting port-forward (API)..."
pkill -f "kubectl.*port-forward.*18080" 2>/dev/null || true
sleep 1
kubectl -n "$NS" port-forward svc/basesource-api-svc 18080:80 > /dev/null 2>&1 &
PF_API_PID=$!
trap "kill $PF_API_PID 2>/dev/null || true" EXIT

for i in $(seq 1 15); do
  curl -s "$API/health" > /dev/null 2>&1 && break
  [ $i -eq 15 ] && { echo "  ❌  API port-forward failed to bind"; exit 1; }
  sleep 1
done
echo "  ✅  API port-forward live ($API)"

if [ $SKIP_RESET -eq 0 ]; then
  echo ""; echo ">>> RESET"
  bash "$DIR/reset.sh"
else
  echo ""; echo ">>> RESET skipped (--no-reset)"
fi

TESTS=(
  "test_01_basic_notification.sh"
  "test_05_durability.sh"
  "test_06_queue_while_offline.sh"
  "test_07_nack.sh"
  "test_08_competing_consumers.sh"
)

PASSED=0; FAILED=0; FAILED_NAMES=()

for t in "${TESTS[@]}"; do
  echo ""; echo ">>> $t"
  if bash "$DIR/$t"; then
    PASSED=$((PASSED+1))
  else
    FAILED=$((FAILED+1))
    FAILED_NAMES+=("$t")
    echo "  ❌  $t FAILED — stopping suite"
    echo "  Debug: kubectl -n $NS logs -l app=basesource-worker --tail=50"
    break
  fi
done

END_TIME=$(date +%s)
echo ""
echo "########################################"
echo "  RESULTS"
echo "########################################"
echo "  Passed : $PASSED / ${#TESTS[@]}"
echo "  Failed : $FAILED"
echo "  Time   : $((END_TIME - START_TIME))s"
echo "########################################"

if [ $FAILED -eq 0 ]; then
  echo ""; echo "  ✅  ALL TESTS PASSED"; echo ""; exit 0
else
  echo ""; echo "  ❌  FAILED: ${FAILED_NAMES[*]}"; echo ""; exit 1
fi
