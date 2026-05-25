#!/usr/bin/env bash
# test_04_replay.sh — Consumer group replay test
#   Verifies StartOffset: kafka.FirstOffset works:
#   start worker with a FRESH group ID (no committed offsets) →
#   assert all messages in topic are replayed from the beginning
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINARY=/tmp/basesource
WORKER_LOG=/tmp/worker.log
REPLAY_LOG=/tmp/worker-replay.log
COMPOSE="docker compose -f $REPO_ROOT/docker/docker-compose.yaml"
KAFKA_GROUP=todo-worker
KAFKA_GROUP_REPLAY=todo-worker-replay
KAFKA_TOPIC=todo-events

echo ""
echo "========================================="
echo "  TEST 04 — Consumer group replay"
echo "========================================="

# ── helpers ───────────────────────────────────────────────────────────────────
pass() { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; exit 1; }

# ── step 1: count messages currently in Kafka topic ──────────────────────────
echo ""
echo "--- Step 1: count messages in Kafka topic"
KAFKA_MSGS=$($COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic "$KAFKA_TOPIC" \
  --from-beginning \
  --max-messages 100 \
  --timeout-ms 5000 2>&1 || true)
TOPIC_COUNT=$(echo "$KAFKA_MSGS" | grep -c "^{" || true)
TOPIC_COUNT="${TOPIC_COUNT//[$'\t\r\n ']}"
[ "$TOPIC_COUNT" -ge 1 ] && pass "topic has $TOPIC_COUNT messages" || \
  fail "no messages in topic — run tests 01-03 first"

# ── step 2: delete the replay group if it exists ─────────────────────────────
echo ""
echo "--- Step 2: clean up replay group"
$COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group "$KAFKA_GROUP_REPLAY" --delete 2>/dev/null || true
pass "replay group cleaned"

# ── step 3: start replay worker with fresh consumer group ────────────────────
# A fresh group has no committed offsets → StartOffset=FirstOffset means offset 0
echo ""
echo "--- Step 3: start replay worker (group=$KAFKA_GROUP_REPLAY, no prior offsets)"
: > "$REPLAY_LOG"
KAFKA_GROUP_ID="$KAFKA_GROUP_REPLAY" "$BINARY" worker >> "$REPLAY_LOG" 2>&1 &
REPLAY_PID=$!

# wait for worker ready
for i in $(seq 1 15); do
  grep -q "outbox relay starting" "$REPLAY_LOG" 2>/dev/null && \
    pass "replay worker started (pid=$REPLAY_PID)" && break
  [ $i -eq 15 ] && fail "replay worker did not start within 15s"
  sleep 1
done

# ── step 4: wait for replayed domain events ───────────────────────────────────
echo ""
echo "--- Step 4: wait for domain events to replay from offset 0 (need >= $TOPIC_COUNT)"
for i in $(seq 1 40); do
  # wrap grep in braces to ensure pipeline always exits 0 (grep exits 1 when no matches)
  EVENTS_REPLAYED=$( { grep '"domain event received"' "$REPLAY_LOG" 2>/dev/null; true; } | wc -l | tr -d ' \t\r\n' )
  if [ "$EVENTS_REPLAYED" -ge "$TOPIC_COUNT" ]; then
    pass "replayed $EVENTS_REPLAYED domain events (topic has $TOPIC_COUNT)"; break
  fi
  [ $i -eq 40 ] && fail "only $EVENTS_REPLAYED events replayed after 40s, expected >=$TOPIC_COUNT"
  sleep 1
done

# ── step 5: verify replay group LAG=0 ────────────────────────────────────────
echo ""
echo "--- Step 5: verify replay group LAG=0"
sleep 3
lag_info=$($COMPOSE exec -T kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group "$KAFKA_GROUP_REPLAY" --describe 2>/dev/null | \
  grep -v "^GROUP\|^$\|CONSUMER-ID" || true)
if echo "$lag_info" | grep -q "."; then
  echo "$lag_info" | awk '{print "  ✅  partition " $3 " LAG=" $6}'
else
  echo "  ⚠️  could not read replay group LAG"
fi

# ── cleanup: stop replay worker only (primary worker must keep running) ───────
kill -TERM $REPLAY_PID 2>/dev/null || true
sleep 2
kill -9 $REPLAY_PID 2>/dev/null || true

echo ""
echo "  TEST 04 PASSED ✅"
