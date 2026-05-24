# feature/996 — Kafka Domain Event Streaming + RabbitMQ Notifications

## Overview

Feature/996 extends feature/997 to add **Kafka** as the domain event streaming backbone while keeping **RabbitMQ** for notification task queuing. Both brokers run simultaneously, each serving its distinct purpose.

| Broker | Purpose | Pattern |
|---|---|---|
| Kafka | Domain event streaming (durable, replayable audit log) | Outbox → Relay → Kafka topic |
| RabbitMQ | Notification task queue (fire-and-forget async tasks) | Usecase → Publisher → Queue |

---

## Architecture

```
HTTP Request
    │
    ▼
TodoUsecase.Create()
    │
    ├─── INSERT todo (same TX)
    ├─── INSERT outbox_events status=pending (same TX)
    └─── notifPublisher.PublishNotification() → RabbitMQ → NotificationConsumer
                                                              └─ logs "notification task received"

[background: OutboxRelay polls every 2s]
    │
    └─── KafkaProducer.Publish(event) → Kafka topic "todo-events"
                                          └─ KafkaConsumer.Start()
                                               └─ logs "domain event received"
```

### Why separate brokers?

- **Kafka** — events are durable and replayable. Consumer groups can replay from offset 0. Good for: audit log, analytics pipeline, event sourcing, multiple downstream consumers.
- **RabbitMQ** — message is consumed once and ACK'd. Good for: task queues, email/push notification jobs, work distribution.

---

## Key Files

| File | Role |
|---|---|
| `internal/infrastructure/messaging/kafka_client.go` | `NewKafkaWriter` / `NewKafkaReader` factory |
| `internal/infrastructure/messaging/kafka_producer.go` | `IEventPublisher` impl — writes to Kafka with aggregate_id as partition key |
| `internal/infrastructure/messaging/kafka_consumer.go` | Reads from Kafka, manual offset commit |
| `internal/infrastructure/messaging/rabbitmq_notification_publisher.go` | `INotificationPublisher` impl — publishes to RabbitMQ default exchange |
| `internal/domain/service/notification_publisher.go` | `INotificationPublisher` interface |
| `internal/domain/service/mock/notification_publisher_mock.go` | Mock for unit tests |
| `internal/infrastructure/messaging/outbox_relay.go` | Polls DB outbox → publishes to Kafka |
| `cmd/worker/cmd.go` | Starts 3 goroutines: outbox relay, Kafka consumer, RabbitMQ consumer |
| `container/container.go` | Wires all dependencies |

---

## Ordering Guarantee

`kafka_producer.go` uses `aggregate_id` (todo ID) as the Kafka message key:

```go
kafka.Message{
    Key:   []byte(event.AggregateID), // same todo always → same partition
    Value: body,
}
```

With `kafka.Hash{}` balancer, all events for the same todo land in the same partition → strict ordering per entity.

---

## Consumer Group Offset

`kafka_client.go` sets `StartOffset: kafka.FirstOffset`:

```go
kafka.ReaderConfig{
    StartOffset: kafka.FirstOffset, // new group reads from beginning
}
```

Without this, a brand-new consumer group starts at `LastOffset` and misses all existing messages.

---

## Scenarios

### Scenario 1: Create Todo — full dual-broker flow

1. `POST /api/v1/todos {"title":"Buy milk"}`
2. Usecase inserts `todos` row + `outbox_events` row (same transaction)
3. Usecase calls `notifPublisher.PublishNotification()` → RabbitMQ queue `todo.notifications`
4. OutboxRelay picks up pending event → `KafkaProducer.Publish()` → Kafka topic `todo-events`
5. KafkaConsumer reads message → logs `domain event received event_type=todo.created`
6. RabbitMQ NotificationConsumer reads task → logs `notification task received`

### Scenario 2: Update/Delete Todo

Same flow as above with `event_type=todo.updated` or `todo.deleted`. No notification is published on update/delete (only on create), so RabbitMQ consumer only fires on creates.

### Scenario 3: Worker restart — Kafka replay

Because `StartOffset: kafka.FirstOffset`, if the consumer group has no committed offset (e.g., first run or group was deleted), the consumer replays all messages from the beginning of the partition. This is the event sourcing / audit log pattern.

### Scenario 4: Kafka broker down

OutboxRelay polls DB every 2s. If Kafka is unreachable, `WriteMessages` returns an error. The outbox row stays `status=pending`. When Kafka recovers, the relay will publish it on the next poll. No message is lost.

### Scenario 5: Duplicate delivery (consumer restart mid-message)

KafkaConsumer uses manual offset commit (`FetchMessage` → `CommitMessages`). If the worker crashes after handling a message but before committing, the message will be re-delivered on restart. Handlers should be idempotent.

---

## Real Test Guide

### Prerequisites

```bash
# Start all infrastructure
docker compose -f docker/docker-compose.yaml up -d db rabbitmq kafka

# Wait for healthy (check)
docker compose -f docker/docker-compose.yaml ps

# Build binary
go build -o /tmp/basesource-996 .

# Run migrations
/tmp/basesource-996 migrate
```

### Start API + Worker

```bash
/tmp/basesource-996 api > /tmp/996-api.log 2>&1 &
/tmp/basesource-996 worker > /tmp/996-worker.log 2>&1 &
```

### Test: Create Todo

```bash
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Kafka live test"}' | jq .
```

### Verify: Worker logs (wait ~3s for relay cycle)

```bash
tail -20 /tmp/996-worker.log
```

Expected:
```
{"msg":"notification task received","to":"admin@example.com","subject":"New Todo Created"}
{"msg":"domain event received","event_type":"todo.created","aggregate_id":"13","event_id":"..."}
```

### Verify: Outbox events published

```bash
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u appuser -papppass appdb \
  -e "SELECT event_id, event_type, status FROM outbox_events ORDER BY created_at DESC LIMIT 5;"
```

Expected: all rows `status=published`.

### Verify: Kafka messages directly

```bash
docker compose -f docker/docker-compose.yaml exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic todo-events \
  --from-beginning \
  --max-messages 10
```

### Verify: RabbitMQ queue via management UI

Open http://localhost:15672 (guest/guest) → Queues → `todo.notifications` → message rate.

### Consumer group offset reset (for replay testing)

```bash
# Kill worker first
pkill -f basesource-996

# Reset offsets to earliest
docker compose -f docker/docker-compose.yaml exec kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group todo-worker \
  --topic todo-events \
  --reset-offsets --to-earliest --execute

# Restart worker — it will replay all past events
/tmp/basesource-996 worker > /tmp/996-worker.log 2>&1 &
```

---

## Verification Checklist

```bash
go build ./...                                    # must pass
go vet ./...                                      # must pass
go test ./internal/usecase/...                    # must pass
grep -r "infrastructure" internal/domain/         # zero results
grep -r "infrastructure" internal/usecase/        # zero results
```
