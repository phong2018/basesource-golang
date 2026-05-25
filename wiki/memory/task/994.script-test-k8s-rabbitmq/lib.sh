#!/usr/bin/env bash
# lib.sh — shared kubectl helpers for k8s integration tests
# Source this file at the top of each test script:
#   source "$(dirname "$0")/lib.sh"

NS=basesource
API=http://localhost:18080
RMQ_API=http://localhost:15672

# ── pod name lookups (fresh each call — pods restart after rollouts) ──────────
mysql_pod()  { kubectl -n "$NS" get pod -l app=mysql     -o jsonpath='{.items[0].metadata.name}' 2>/dev/null; }
kafka_pod()  { kubectl -n "$NS" get pod -l app=kafka     -o jsonpath='{.items[0].metadata.name}' 2>/dev/null; }
rmq_pod()    { kubectl -n "$NS" get pod -l app=rabbitmq  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null; }

# ── database ──────────────────────────────────────────────────────────────────
db_exec() {
  kubectl -n "$NS" exec "$(mysql_pod)" -- \
    mysql -u appuser -papppass appdb -sNe "$1" 2>/dev/null
}

db_delivery_status() {
  local agg=$1 event_type=$2 dest=$3
  db_exec "SELECT d.status FROM outbox_events e
           JOIN outbox_deliveries d ON d.outbox_event_id = e.id
           WHERE e.aggregate_id='$agg' AND e.event_type='$event_type'
           AND d.destination='$dest'
           ORDER BY d.id DESC LIMIT 1;" | tr -d ' \t\r\n'
}

wait_for_delivery() {
  local agg=$1 event_type=$2 dest=$3 expected=$4 timeout=${5:-15}
  local i=0
  while true; do
    got=$(db_delivery_status "$agg" "$event_type" "$dest")
    [ "$got" = "$expected" ] && pass "outbox[$agg/$event_type] $dest=$expected" && return
    [ $i -ge $timeout ] && fail "outbox[$agg/$event_type] $dest: want $expected, got '$got'"
    sleep 1; i=$((i+1))
  done
}

# ── kafka ─────────────────────────────────────────────────────────────────────
kafka_exec() {
  kubectl -n "$NS" exec "$(kafka_pod)" -- "$@" 2>/dev/null
}

# ── worker logs ───────────────────────────────────────────────────────────────
# Combined logs from all worker pods (interleaved).
worker_logs() {
  kubectl -n "$NS" logs -l app=basesource-worker --tail=1000 2>/dev/null || true
}

# Logs from a specific pod by name.
pod_logs() {
  local pod=$1
  kubectl -n "$NS" logs "$pod" --tail=500 2>/dev/null || true
}

# Wait for a pattern to appear in combined worker logs.
wait_for_worker_log() {
  local pattern=$1 timeout=${2:-30}
  local i=0
  while ! worker_logs | grep -q "$pattern"; do
    [ $i -ge $timeout ] && fail "timeout (${timeout}s) — pattern not found in worker logs: $pattern"
    sleep 1; i=$((i+1))
  done
  pass "worker log: $pattern"
}

# Wait for aggregate_id + event_type on same log line.
wait_for_worker_log_agg() {
  local agg=$1 event_type=$2 timeout=${3:-30}
  local i=0
  while ! worker_logs | grep "\"aggregate_id\":\"$agg\"" | grep -q "\"event_type\":\"$event_type\""; do
    [ $i -ge $timeout ] && fail "timeout (${timeout}s) — no $event_type for aggregate $agg in worker logs"
    sleep 1; i=$((i+1))
  done
  pass "worker log: $event_type for aggregate $agg"
}

# Count occurrences of a pattern in combined worker logs.
worker_log_count() {
  local pattern=$1
  worker_logs | { grep -c "$pattern" || true; } | tr -d ' \t\r\n'
}

# ── scaling ───────────────────────────────────────────────────────────────────
scale_deployment() {
  local name=$1 replicas=$2
  kubectl -n "$NS" scale deployment "$name" --replicas="$replicas"
  if [ "$replicas" -gt 0 ]; then
    kubectl -n "$NS" wait --for=condition=ready pod -l "app=$name" --timeout=60s
  else
    kubectl -n "$NS" wait --for=delete pod -l "app=$name" --timeout=60s 2>/dev/null || true
  fi
}

restart_worker() {
  kubectl -n "$NS" rollout restart deployment basesource-worker
  kubectl -n "$NS" rollout status deployment basesource-worker --timeout=60s
}

# ── rabbitmq helpers (via rabbitmqctl inside the pod) ─────────────────────────
# rabbitmqctl is always available in the rabbitmq image; no curl/wget needed.

_rmq_queue_field() {
  local queue=$1 field=$2
  kubectl -n "$NS" exec "$(rmq_pod)" -- \
    rabbitmqctl list_queues -q name "$field" 2>/dev/null | \
    awk -v q="$queue" '$1==q {print $2}' | tr -d ' \t\r\n'
}

rmq_queue_depth() {
  local v
  v=$(_rmq_queue_field "todo.notifications" "messages")
  echo "${v:-0}"
}

rmq_consumers() {
  local v
  v=$(_rmq_queue_field "todo.notifications" "consumers")
  echo "${v:-0}"
}

rmq_unacked() {
  local v
  v=$(_rmq_queue_field "todo.notifications" "messages_unacknowledged")
  echo "${v:-0}"
}

# Inject a message into an exchange via rabbitmqadmin inside the pod.
rmq_publish_via_api() {
  local exchange=$1 routing_key=$2 payload=$3
  kubectl -n "$NS" exec "$(rmq_pod)" -- \
    rabbitmqadmin publish "exchange=${exchange}" "routing_key=${routing_key}" "payload=${payload}" \
    > /dev/null 2>/dev/null
}

# Wait for RabbitMQ to be ready (AMQP + rabbitmqctl) after a scale-up.
wait_for_rmq_ready() {
  local timeout=${1:-60}
  local i=0
  while true; do
    result=$(kubectl -n "$NS" exec "$(rmq_pod)" -- \
      rabbitmq-diagnostics ping 2>/dev/null | grep -c "Ping succeeded" || true)
    [ "$result" -ge 1 ] && pass "RabbitMQ ready" && return
    [ $i -ge $timeout ] && fail "RabbitMQ did not become ready within ${timeout}s"
    sleep 1; i=$((i+1))
  done
}

# ── output helpers ────────────────────────────────────────────────────────────
pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }
