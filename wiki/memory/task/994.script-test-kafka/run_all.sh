#!/usr/bin/env bash
# run_all.sh — run reset + all integration tests in sequence
# Usage:
#   bash run_all.sh            # full suite (reset + 4 tests)
#   bash run_all.sh --no-reset # skip reset (use existing binary + infra)
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
START_TIME=$(date +%s)

SKIP_RESET=0
for arg in "$@"; do
  [ "$arg" = "--no-reset" ] && SKIP_RESET=1
done

echo ""
echo "########################################"
echo "  Kafka Integration Test Suite"
echo "########################################"
echo "  Working dir : $(pwd)"
echo "  Scripts dir : $DIR"
echo "  Date        : $(date)"
echo "########################################"

# ── reset ─────────────────────────────────────────────────────────────────────
if [ $SKIP_RESET -eq 0 ]; then
  echo ""
  echo ">>> RESET"
  bash "$DIR/reset.sh"
else
  echo ""
  echo ">>> RESET skipped (--no-reset)"
fi

# ── tests ─────────────────────────────────────────────────────────────────────
TESTS=(
  "test_01_create.sh"
  "test_02_update.sh"
  "test_03_delete.sh"
  "test_04_replay.sh"
)

PASSED=0
FAILED=0
FAILED_NAMES=()

for t in "${TESTS[@]}"; do
  echo ""
  echo ">>> $t"
  if bash "$DIR/$t"; then
    PASSED=$((PASSED+1))
  else
    FAILED=$((FAILED+1))
    FAILED_NAMES+=("$t")
    echo "  ❌  $t FAILED — stopping suite"
    break
  fi
done

# ── summary ───────────────────────────────────────────────────────────────────
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "########################################"
echo "  RESULTS"
echo "########################################"
echo "  Passed : $PASSED"
echo "  Failed : $FAILED"
echo "  Time   : ${ELAPSED}s"
echo "########################################"

if [ $FAILED -eq 0 ]; then
  echo ""
  echo "  ✅  ALL TESTS PASSED"
  echo ""
  # cleanup: kill background processes
  pkill -f "/tmp/basesource" 2>/dev/null || true
  exit 0
else
  echo ""
  echo "  ❌  FAILED: ${FAILED_NAMES[*]}"
  echo ""
  echo "  Logs:"
  echo "    API    : /tmp/api.log"
  echo "    Worker : /tmp/worker.log"
  echo ""
  exit 1
fi
