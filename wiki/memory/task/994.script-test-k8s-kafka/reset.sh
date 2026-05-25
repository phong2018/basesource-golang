#!/usr/bin/env bash
# reset.sh — wipe DB, reset Kafka topic + consumer group, restart worker pods
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

NS=basesource
KAFKA_TOPIC=todo-events
KAFKA_GROUP=todo-worker

info() { echo "[reset] $*"; }

echo ""
echo "========================================="
echo "  RESET — Kafka Suite"
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

info "deleting consumer group '$KAFKA_GROUP'..."
kafka_exec /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group "$KAFKA_GROUP" --delete 2>/dev/null || true
pass "consumer group deleted (or did not exist)"

info "deleting Kafka topic '$KAFKA_TOPIC'..."
kafka_exec /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --delete --topic "$KAFKA_TOPIC" 2>/dev/null || true
for i in $(seq 1 10); do
  exists=$(kafka_exec /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server localhost:9092 --list 2>/dev/null | { grep -c "^${KAFKA_TOPIC}$" || true; })
  exists="${exists//[$'\t\r\n ']}"
  [ "$exists" = "0" ] && break
  sleep 1
done
pass "Kafka topic deleted"

info "recreating Kafka topic '$KAFKA_TOPIC' with 1 partition..."
kafka_exec /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --topic "$KAFKA_TOPIC" \
  --partitions 1 --replication-factor 1 2>/dev/null || true
pass "Kafka topic recreated"

info "restarting worker pods..."
restart_worker
pass "worker pods restarted"

info "waiting for consumer group to stabilize..."
for i in $(seq 1 60); do
  cg_out=$(kafka_exec /opt/kafka/bin/kafka-consumer-groups.sh \
    --bootstrap-server localhost:9092 \
    --group "$KAFKA_GROUP" --describe 2>/dev/null || true)
  member_count=$(echo "$cg_out" | grep -v "^GROUP\|^$\|CONSUMER-ID\|Error" | { grep -c "." || true; })
  member_count="${member_count//[$'\t\r\n ']}"
  if [ "$member_count" = "1" ]; then
    pass "consumer group stable (1 partition assigned)"; break
  fi
  [ $i -eq 60 ] && { echo "  ⚠️  consumer group not stable after 120s (rows=$member_count), proceeding anyway"; break; }
  sleep 2
done

echo ""
echo "========================================="
echo "  RESET COMPLETE — ready to run tests"
echo "========================================="
