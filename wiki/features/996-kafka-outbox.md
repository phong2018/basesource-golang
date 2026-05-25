# feature/996 — Dual Relay: Kafka + RabbitMQ Outbox

## Overview

Both Kafka and RabbitMQ are driven by the **same outbox pattern**, but with independent delivery tracking. Each event written in a usecase transaction gets one `outbox_deliveries` row per broker. The two relays poll and publish completely independently — one broker's failure has no impact on the other.

| Broker | Purpose | Pattern |
|---|---|---|
| Kafka | Domain event streaming (durable, replayable audit log) | `outbox_deliveries destination=kafka` → `KafkaOutboxRelay` → Kafka topic |
| RabbitMQ | Notification task queue (fire-and-forget async tasks) | `outbox_deliveries destination=rabbitmq` → `RabbitMQOutboxRelay` → RabbitMQ exchange |

---

## Architecture

```
HTTP Request
    │
    ▼
TodoUsecase.Create()
    │
    ├─── INSERT todos              ┐
    ├─── INSERT audit_logs         ├ one atomic DB transaction
    ├─── INSERT outbox_events      │
    └─── INSERT outbox_deliveries  ┘
              ├── (destination=kafka,    status=pending)
              └── (destination=rabbitmq, status=pending)

[background workers — fully independent]

KafkaOutboxRelay (every 2s)
    SELECT outbox_deliveries WHERE destination='kafka' AND status='pending'
    → GetEventByID()
    → KafkaProducer.Publish() → Kafka topic "todo-events"
    → MarkDeliveryPublished()
        └─ KafkaConsumer.Start()
             └─ DomainEventHandler.Handle() → logs "domain event received"

RabbitMQOutboxRelay (every 2s)
    SELECT outbox_deliveries WHERE destination='rabbitmq' AND status='pending'
    → GetEventByID()
    → RabbitMQPublisher.Publish() → RabbitMQ exchange "todo.events"
    → MarkDeliveryPublished()
        └─ RabbitMQNotificationConsumer.Start()
             └─ HandleNotificationTask() → logs "notification task received"
```

### Why separate brokers?

- **Kafka** — events are durable and replayable. Consumer groups can replay from offset 0. Good for: audit log, analytics pipeline, event sourcing, multiple downstream consumers.
- **RabbitMQ** — message is consumed once and ACK'd. Good for: task queues, email/push notification jobs, work distribution.

---

## Schema

### `outbox_events`

```sql
CREATE TABLE outbox_events (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_id       VARCHAR(36)  NOT NULL UNIQUE,
    event_type     VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64)  NOT NULL,
    aggregate_id   VARCHAR(64)  NOT NULL,
    payload        JSON         NOT NULL,
    created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);
```

### `outbox_deliveries`

```sql
CREATE TABLE outbox_deliveries (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    outbox_event_id BIGINT UNSIGNED NOT NULL,
    destination     VARCHAR(32)  NOT NULL,   -- 'kafka' | 'rabbitmq'
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempt_count   INT UNSIGNED NOT NULL DEFAULT 0,
    last_error      TEXT,
    published_at    DATETIME(3),
    created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    CONSTRAINT fk_od_event FOREIGN KEY (outbox_event_id) REFERENCES outbox_events(id),
    INDEX idx_od_dest_status (destination, status, id)
);
```

---

## Domain model

### `OutboxEvent` (`internal/domain/model/event.go`)

```go
type OutboxEvent struct {
    ID            uint      `db:"id"`
    EventID       string    `db:"event_id"`
    EventType     string    `db:"event_type"`
    AggregateType string    `db:"aggregate_type"`
    AggregateID   string    `db:"aggregate_id"`
    Payload       string    `db:"payload"`
    CreatedAt     time.Time `db:"created_at"`
}
```

### `OutboxDelivery` (`internal/domain/model/event.go`)

```go
type OutboxDelivery struct {
    ID            uint       `db:"id"`
    OutboxEventID uint       `db:"outbox_event_id"`
    Destination   string     `db:"destination"`
    Status        string     `db:"status"`
    AttemptCount  int        `db:"attempt_count"`
    LastError     *string    `db:"last_error"`
    PublishedAt   *time.Time `db:"published_at"`
    CreatedAt     time.Time  `db:"created_at"`
    UpdatedAt     time.Time  `db:"updated_at"`
}
```

### Constants (`internal/domain/model/event_constant.go`)

```go
const (
    OutboxDestinationKafka    = "kafka"
    OutboxDestinationRabbitMQ = "rabbitmq"

    OutboxDeliveryStatusPending   = "pending"
    OutboxDeliveryStatusPublished = "published"
    OutboxDeliveryStatusFailed    = "failed"
)
```

---

## Repository interface (`IOutboxRepository`)

```go
type IOutboxRepository interface {
    // write path — called inside a usecase transaction
    CreateEventWithDeliveries(ctx context.Context, event *model.OutboxEvent, destinations []string) error

    // relay read path — scoped per destination broker
    ListPendingDeliveries(ctx context.Context, destination string, limit int) ([]*model.OutboxDelivery, error)
    GetEventByID(ctx context.Context, id uint) (*model.OutboxEvent, error)

    // relay update path
    MarkDeliveryPublished(ctx context.Context, deliveryID uint) error
    MarkDeliveryFailed(ctx context.Context, deliveryID uint, errMsg string) error
}
```

---

## Relay constructors

One generic `OutboxRelay` struct; two named constructors so call sites are self-documenting:

```go
// NewKafkaOutboxRelay — hard-codes destination="kafka"
func NewKafkaOutboxRelay(repo IOutboxRepository, publisher IEventPublisher) *OutboxRelay

// NewRabbitMQOutboxRelay — hard-codes destination="rabbitmq"
func NewRabbitMQOutboxRelay(repo IOutboxRepository, publisher IEventPublisher) *OutboxRelay
```

Container wiring (`container/container.go`):

```go
KafkaOutboxRelay    = messaging.NewKafkaOutboxRelay(outboxRepo, kafkaProducer)
RabbitMQOutboxRelay = messaging.NewRabbitMQOutboxRelay(outboxRepo, rmqPublisher)
```

---

## Worker goroutines (`cmd/worker/cmd.go`)

```
goroutine 1: KafkaOutboxRelay.Start()     — outbox → Kafka
goroutine 2: RabbitMQOutboxRelay.Start()  — outbox → RabbitMQ
goroutine 3: KafkaConsumer.Start()        — Kafka → DomainEventHandler
goroutine 4: RabbitMQNotificationConsumer.Start() — RabbitMQ → HandleNotificationTask
```

---

## Key files

| File | Role |
|---|---|
| `internal/domain/model/event.go` | `OutboxEvent` + `OutboxDelivery` entities |
| `internal/domain/model/event_constant.go` | Destination + delivery status constants |
| `internal/domain/repository/outbox_repository.go` | `IOutboxRepository` 5-method interface |
| `internal/domain/repository/mock/outbox_repository_mock.go` | Mock for unit tests |
| `internal/infrastructure/repository/outbox_repository_impl.go` | SQL impl of `IOutboxRepository` |
| `internal/infrastructure/messaging/outbox_relay.go` | `OutboxRelay` struct; `NewKafkaOutboxRelay` / `NewRabbitMQOutboxRelay` |
| `internal/infrastructure/messaging/kafka_client.go` | `NewKafkaWriter` / `NewKafkaReader` factory |
| `internal/infrastructure/messaging/kafka_producer.go` | `IEventPublisher` impl for Kafka |
| `internal/infrastructure/messaging/kafka_consumer.go` | Reads from Kafka, manual offset commit |
| `internal/infrastructure/messaging/rabbitmq_publisher.go` | `IEventPublisher` impl for RabbitMQ |
| `internal/infrastructure/messaging/rabbitmq_consumer.go` | Durable queue, ACK/NACK |
| `container/container.go` | Wires both relay instances independently |
| `cmd/worker/cmd.go` | Starts 4 goroutines |
| `db/migrations/20260524000003_create_outbox_events_table.sql` | `outbox_events` table |
| `db/migrations/20260525000005_create_outbox_deliveries.sql` | `outbox_deliveries` table |

---

## Ordering guarantee (Kafka)

`kafka_producer.go` uses `AggregateID` as the Kafka message key:

```go
kafka.Message{
    Key:   []byte(event.AggregateID), // same todo always → same partition
    Value: body,
}
```

With `kafka.Hash{}` balancer, all events for the same todo land in the same partition → strict ordering per entity.

---

## Consumer group offset (Kafka)

`kafka_client.go` sets `StartOffset: kafka.FirstOffset`:

```go
kafka.ReaderConfig{
    StartOffset: kafka.FirstOffset, // new group reads from beginning
}
```

Without this, a brand-new consumer group starts at `LastOffset` and misses all existing messages.

---

## Scenarios

### Scenario 1: Create Todo — dual broker flow

1. `POST /api/v1/todos {"title":"Buy milk"}`
2. Usecase transaction: INSERT `todos` + `outbox_events` + two `outbox_deliveries` rows
3. `KafkaOutboxRelay` picks up the kafka delivery → publishes to topic `todo-events`
4. `RabbitMQOutboxRelay` picks up the rabbitmq delivery → publishes to exchange `todo.events`
5. `KafkaConsumer` reads message → logs `domain event received event_type=todo.created`
6. `RabbitMQNotificationConsumer` reads task → logs `notification task received`

### Scenario 2: Kafka down, RabbitMQ healthy

- `KafkaOutboxRelay` fails → kafka delivery stays `pending`, `last_error` recorded
- `RabbitMQOutboxRelay` succeeds → rabbitmq delivery becomes `published`
- When Kafka recovers, `KafkaOutboxRelay` retries the pending delivery on the next tick

### Scenario 3: Worker restart — Kafka replay

Because `StartOffset: kafka.FirstOffset`, if the consumer group has no committed offset (first run or group deleted), the consumer replays all messages from the beginning. This is the event sourcing / audit log pattern.

### Scenario 4: Duplicate delivery (consumer restart mid-message)

`KafkaConsumer` uses manual offset commit (`FetchMessage` → `CommitMessages`). If the worker crashes after handling a message but before committing, the message will be re-delivered on restart. Handlers must be idempotent.

---

## Real Test Guide

### Prerequisites

```bash
docker compose -f docker/docker-compose.yaml up -d db rabbitmq kafka
go build -o /tmp/basesource .
/tmp/basesource migrate
```

### Start API + Worker

```bash
/tmp/basesource api    > /tmp/api.log    2>&1 &
/tmp/basesource worker > /tmp/worker.log 2>&1 &
```

### Test: Create Todo

```bash
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Dual relay test"}' | jq .
```

### Verify: Worker logs (wait ~3s for relay cycle)

```bash
tail -20 /tmp/worker.log
```

Expected:
```
{"msg":"notification task received","to":"admin@example.com","subject":"New Todo Created"}
{"msg":"domain event received","event_type":"todo.created","aggregate_id":"1"}
```

### Verify: Delivery status in DB

```bash
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u appuser -papppass appdb \
  -e "SELECT e.event_type, d.destination, d.status, d.attempt_count FROM outbox_events e JOIN outbox_deliveries d ON d.outbox_event_id = e.id ORDER BY e.id, d.destination;" \
  2>/dev/null | grep -v Warning
```

Expected:
```
event_type    destination  status     attempt_count
todo.created  kafka        published  1
todo.created  rabbitmq     published  1
```

### Verify: Kafka messages directly

```bash
docker compose -f docker/docker-compose.yaml exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic todo-events \
  --from-beginning \
  --max-messages 10
```

### Consumer group offset reset (for replay testing)

```bash
pkill -f basesource

docker compose -f docker/docker-compose.yaml exec kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group todo-worker \
  --topic todo-events \
  --reset-offsets --to-earliest --execute

/tmp/basesource worker > /tmp/worker.log 2>&1 &
```

---

## Verification Checklist

```bash
go build ./...                                    # must pass
go vet ./...                                      # must pass
go test ./internal/usecase/...                    # must pass
go test ./internal/infrastructure/messaging/... -short  # must pass
grep -r "infrastructure" internal/domain/         # zero results
grep -r "infrastructure" internal/usecase/        # zero results
```
