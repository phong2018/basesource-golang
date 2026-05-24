#!/usr/bin/env bash
# run_all.sh — run the full RabbitMQ integration test suite
set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPTS_DIR/../../../.." && pwd)"
BINARY=/tmp/basesource

PASSED=0
FAILED=0
FAILED_TESTS=()
START_TIME=$(date +%s)

echo "########################################"
echo "  RabbitMQ Integration Test Suite"
echo "########################################"
echo "  Working dir : $REPO_ROOT"
echo "  Scripts dir : $SCRIPTS_DIR"
echo "  Date        : $(date)"
echo "########################################"
echo ""

# ── reset ─────────────────────────────────────────────────────────────────────
echo ">>> RESET"
bash "$SCRIPTS_DIR/reset.sh"

# ── test runner ───────────────────────────────────────────────────────────────
run_test() {
  local script=$1
  echo ""
  echo ">>> $script"
  if bash "$SCRIPTS_DIR/$script"; then
    PASSED=$(( PASSED + 1 ))
  else
    FAILED=$(( FAILED + 1 ))
    FAILED_TESTS+=("$script")
    echo ""
    echo "  FAILED — stopping suite"
    echo ""
    print_results
    # cleanup: kill any leftover processes
    pkill -9 -f "$BINARY worker" 2>/dev/null || true
    exit 1
  fi

  # after tests 05/06/07 the worker was restarted; ensure it's still alive
  # for the next test before continuing
  if ! pgrep -f "$BINARY worker" > /dev/null 2>&1; then
    echo "  [run_all] worker not running after $script — restarting..."
    WORKER_LOG=/tmp/worker.log
    : > "$WORKER_LOG"
    "$BINARY" worker >> "$WORKER_LOG" 2>&1 &
    for i in $(seq 1 20); do
      grep -q "rabbitmq notification consumer starting" "$WORKER_LOG" 2>/dev/null && break
      [ $i -eq 20 ] && echo "  [run_all] ❌  worker failed to restart" && exit 1
      sleep 1
    done
    echo "  [run_all] ✅  worker restarted"
  fi
}

print_results() {
  END_TIME=$(date +%s)
  ELAPSED=$(( END_TIME - START_TIME ))
  echo "########################################"
  echo "  RESULTS"
  echo "########################################"
  echo "  Passed : $PASSED"
  echo "  Failed : $FAILED"
  echo "  Time   : ${ELAPSED}s"
  if [ "${#FAILED_TESTS[@]}" -gt 0 ]; then
    echo "  Failed tests:"
    for t in "${FAILED_TESTS[@]}"; do echo "    ❌  $t"; done
  fi
  echo "########################################"
}

run_test test_05_durability.sh
run_test test_06_queue_while_offline.sh
run_test test_07_nack.sh
run_test test_08_competing_consumers.sh

# ── cleanup ───────────────────────────────────────────────────────────────────
pkill -9 -f "$BINARY worker" 2>/dev/null || true

echo ""
print_results
echo ""
echo "  ✅  ALL TESTS PASSED"
