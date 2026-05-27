#!/usr/bin/env bash
# reset.sh — wipe DB, restart worker pods (no Kafka topic reset needed for RabbitMQ suite)
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

NS=basesource

info() { echo "[reset] $*"; }

echo ""
echo "========================================="
echo "  RESET — RabbitMQ Suite"
echo "========================================="

info "checking cluster..."
kubectl -n "$NS" get pods > /dev/null
pass "cluster reachable"

info "checking API port-forward..."
for i in $(seq 1 10); do
  curl -s "$API/health" > /dev/null 2>&1 && pass "API port-forward live ($API)" && break
  [ $i -eq 10 ] && fail "API port-forward not live — run_all.sh must start port-forwards first"
  sleep 1
done

info "truncating DB tables..."
db_exec "SET FOREIGN_KEY_CHECKS=0; TRUNCATE TABLE outbox_deliveries; TRUNCATE TABLE outbox_events; TRUNCATE TABLE audit_logs; TRUNCATE TABLE todos; SET FOREIGN_KEY_CHECKS=1;"
pass "DB tables truncated"

info "purging RabbitMQ queue (removes leftover messages from K6/previous runs)..."
kubectl -n "$NS" exec "$(rmq_pod)" -- rabbitmqctl purge_queue todo.notifications > /dev/null 2>&1 || true
pass "RabbitMQ queue purged"

info "restarting worker pods (clears in-memory relay state)..."
restart_worker
pass "worker pods restarted"

info "waiting for worker pods to subscribe to RabbitMQ queue..."
for i in $(seq 1 30); do
  C=$(rmq_consumers); C="${C//[$'\t\r\n ']}"
  [ "$C" -ge 1 ] && pass "RabbitMQ queue has $C consumer(s)" && break
  [ $i -eq 30 ] && fail "no consumers on RabbitMQ queue after 30s"
  sleep 1
done

echo ""
echo "========================================="
echo "  RESET COMPLETE — ready to run tests"
echo "========================================="
