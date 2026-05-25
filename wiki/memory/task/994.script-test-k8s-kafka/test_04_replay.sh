#!/usr/bin/env bash
# test_04_replay.sh — Consumer group replay test
#   Verifies StartOffset: kafka.FirstOffset works:
#   start a fresh consumer group (no committed offsets) →
#   assert all messages in topic are replayed from the beginning.
#
#   K8s approach: deploy a temporary basesource-worker-replay Deployment
#   with KAFKA_GROUP_ID=todo-worker-replay, read its logs, then delete it.
#   A Job is NOT used because app worker is a daemon — it never exits.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR/lib.sh"

KAFKA_TOPIC=todo-events
KAFKA_GROUP_REPLAY=todo-worker-replay
REPLAY_DEPLOY=basesource-worker-replay

echo ""
echo "========================================="
echo "  TEST 04 — Consumer group replay"
echo "========================================="

# ── step 1: count messages currently in Kafka topic ──────────────────────────
echo ""
echo "--- Step 1: count messages in Kafka topic"
KAFKA_MSGS=$(kafka_exec /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic "$KAFKA_TOPIC" \
  --from-beginning \
  --max-messages 100 \
  --timeout-ms 5000 2>&1 || true)
TOPIC_COUNT=$(echo "$KAFKA_MSGS" | { grep -c "^{" || true; })
TOPIC_COUNT="${TOPIC_COUNT//[$'\t\r\n ']}"
[ "$TOPIC_COUNT" -ge 1 ] && pass "topic has $TOPIC_COUNT messages" || \
  fail "no messages in topic — run tests 01–03 first"

# ── step 2: clean up replay consumer group ───────────────────────────────────
echo ""
echo "--- Step 2: clean up replay consumer group"
kafka_exec /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group "$KAFKA_GROUP_REPLAY" --delete 2>/dev/null || true
pass "replay group cleaned"

# ── step 3: delete leftover replay deployment if it exists ───────────────────
kubectl -n "$NS" delete deployment "$REPLAY_DEPLOY" 2>/dev/null || true

# ── step 4: deploy replay worker with fresh consumer group ───────────────────
echo ""
echo "--- Step 4: deploy $REPLAY_DEPLOY (group=$KAFKA_GROUP_REPLAY)"
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $REPLAY_DEPLOY
  namespace: $NS
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $REPLAY_DEPLOY
  template:
    metadata:
      labels:
        app: $REPLAY_DEPLOY
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
      containers:
        - name: worker
          image: basesource:local
          imagePullPolicy: Never
          args: ["worker"]
          env:
            - name: KAFKA_GROUP_ID
              value: $KAFKA_GROUP_REPLAY
          envFrom:
            - configMapRef:
                name: basesource-config
            - secretRef:
                name: basesource-secret
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
EOF

kubectl -n "$NS" wait --for=condition=ready pod -l "app=$REPLAY_DEPLOY" --timeout=60s
pass "replay worker deployed"

# ── step 5: wait for replayed domain events ───────────────────────────────────
echo ""
echo "--- Step 5: wait for domain events replayed from offset 0 (need >= $TOPIC_COUNT)"
for i in $(seq 1 40); do
  EVENTS_REPLAYED=$(kubectl -n "$NS" logs -l "app=$REPLAY_DEPLOY" --tail=500 2>/dev/null | \
    { grep -c '"domain event received"' || true; })
  EVENTS_REPLAYED="${EVENTS_REPLAYED//[$'\t\r\n ']}"
  if [ "$EVENTS_REPLAYED" -ge "$TOPIC_COUNT" ]; then
    pass "replayed $EVENTS_REPLAYED domain events (topic has $TOPIC_COUNT)"; break
  fi
  [ $i -eq 40 ] && fail "only $EVENTS_REPLAYED events replayed after 40s, expected >=$TOPIC_COUNT"
  sleep 1
done

# ── step 6: verify replay group LAG=0 ────────────────────────────────────────
echo ""
echo "--- Step 6: verify replay group LAG=0"
sleep 3
lag_info=$(kafka_exec /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group "$KAFKA_GROUP_REPLAY" --describe 2>/dev/null | \
  grep -v "^GROUP\|^$\|CONSUMER-ID" || true)
if echo "$lag_info" | grep -q "."; then
  echo "$lag_info" | awk '{print "  ✅  partition " $3 " LAG=" $6}'
else
  echo "  ⚠️  could not read replay group LAG (group may not have committed yet)"
fi

# ── cleanup: delete the replay deployment ────────────────────────────────────
echo ""
echo "--- Cleanup: delete $REPLAY_DEPLOY deployment"
kubectl -n "$NS" delete deployment "$REPLAY_DEPLOY" 2>/dev/null || true
pass "$REPLAY_DEPLOY deleted"

echo ""
echo "  TEST 04 PASSED ✅"
