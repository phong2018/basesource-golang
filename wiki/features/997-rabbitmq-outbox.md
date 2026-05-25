# feature/997 — RabbitMQ + Outbox Pattern

## What problem does this solve?

If you publish directly to RabbitMQ **after** `tx.Commit()`, a process crash in that gap permanently loses the event — there is no way to recover it.

```
// UNSAFE — direct publish
tx.Commit()                // ← todo saved to DB
broker.Publish(event)      // ← crash here → event lost forever
```

The **outbox pattern** fixes this by writing the event inside the same DB transaction as the business data. Even if the process crashes immediately after commit, the event row survives in MySQL with `status=pending` and will be picked up when the process restarts.

```
// SAFE — outbox pattern
tx.WithinTransaction():
    INSERT todos                ┐
    INSERT audit_logs           ├ one atomic transaction
    INSERT outbox_events        ┘
    INSERT outbox_deliveries        (destination='rabbitmq', status=pending)

(process can crash here — rows are already in MySQL)

OutboxRelay wakes up every 2s:
    SELECT outbox_deliveries WHERE destination='rabbitmq' AND status='pending'
    → publish to RabbitMQ → mark delivery published
```

---

## Architecture

```
[API server process]                 [Worker process]
─────────────────────                ──────────────────────────────────────────────
POST /todos                          goroutine 1: KafkaOutboxRelay
  └─ usecase.Create()                  └─ every 2s: processes kafka deliveries
       └─ WithinTransaction()
            ├─ INSERT todos          goroutine 2: RabbitMQOutboxRelay
            ├─ INSERT audit_logs       └─ every 2s:
            ├─ INSERT outbox_events         SELECT outbox_deliveries
            └─ INSERT outbox_deliveries       WHERE destination='rabbitmq'
               (kafka, pending)               AND status='pending'
               (rabbitmq, pending)          publisher.Publish(event) → RabbitMQ
                                            UPDATE status='published'
                    ↕ MySQL
                                       goroutine 3: KafkaConsumer
                    ↕ RabbitMQ           └─ reads from Kafka topic

                                       goroutine 4: RabbitMQNotificationConsumer
                                          └─ queue: todo.notifications
                                             handler(body) → log / process
                                             Ack() on success
                                             Nack(requeue=false) → dead-letter
```

**Key rule:** the usecase writes only to MySQL. It never calls RabbitMQ directly. The relay worker is the only component that talks to the broker.

---

## Dual-broker delivery table

Each `outbox_events` row has **one `outbox_deliveries` row per destination**. The relays are fully independent — a RabbitMQ failure does not affect Kafka delivery and vice versa.

```
outbox_events
  id=1, event_type=todo.created

outbox_deliveries
  event_id=1, destination=kafka,    status=published
  event_id=1, destination=rabbitmq, status=pending   ← retries independently
```

---

## Publisher vs Consumer

### Publisher — the RabbitMQ outbox relay

`RabbitMQOutboxRelay` is constructed with `NewRabbitMQOutboxRelay` in `container/container.go`.
It uses a **dedicated** `RabbitMQPublisherClient` (separate from the consumer's client) so reconnects never interfere with each other.

- Polls `outbox_deliveries WHERE destination='rabbitmq' AND status='pending'` every 2 seconds
- Calls `rabbitmq_publisher.Publish(event)` → sends to exchange `todo.events` with routing key = `event.EventType` (e.g. `todo.created`)
- On success: marks delivery `published`, records `published_at`, increments `attempt_count`
- On transient publish failure (e.g. broker down): leaves delivery as `pending` — **no** `MarkDeliveryFailed` call; the relay retries on the next 2-second tick
- `rabbitmqPublisher.Publish()` itself tries one automatic reconnect before returning the error, so a broker restart is recovered on the next relay tick

```
usecase.Create()
    INSERT outbox_events + outbox_deliveries (status=pending)   ← usecase writes here

RabbitMQOutboxRelay (2s later)
    SELECT delivery WHERE destination='rabbitmq' AND status='pending'
    → GetEventByID()
    → rabbitmq_publisher.Publish()
         ├─ success  → MarkDeliveryPublished()
         └─ error    → try client.Reconnect() → retry Publish()
                           ├─ success → MarkDeliveryPublished()
                           └─ error   → log warning, leave delivery pending (retry next tick)
```

### Consumer — the notification handler

`RabbitMQNotificationConsumer` runs in goroutine 4 of the worker.
It uses a **dedicated** `RabbitMQConsumerClient` (separate from the publisher's client).

- Queue: `todo.notifications`
- `prefetch=1` — processes one message at a time (fair dispatch)
- Calls `HandleNotificationTask(body)` for each message
- Sends `Ack()` when handler succeeds → RabbitMQ removes the message
- Sends `Nack(requeue=false)` when handler fails → message goes to dead-letter queue
- **Auto-reconnect**: when the broker closes the channel (`!ok`), the internal goroutine waits 3 seconds, calls `client.Reconnect()`, re-declares the queue and binding, and resumes consuming — no worker restart required

---

## Key files

| File | Role |
|---|---|
| `internal/domain/model/event.go` | `OutboxEvent` + `OutboxDelivery` entities |
| `internal/domain/model/event_constant.go` | Destination constants (`kafka`, `rabbitmq`) + delivery status constants |
| `internal/domain/repository/outbox_repository.go` | `IOutboxRepository` — 5-method interface |
| `internal/domain/repository/mock/outbox_repository_mock.go` | Mock for unit tests |
| `internal/infrastructure/repository/outbox_repository_impl.go` | DB impl: `CreateEventWithDeliveries`, `ListPendingDeliveries`, `GetEventByID`, `MarkDeliveryPublished`, `MarkDeliveryFailed` |
| `internal/infrastructure/messaging/outbox_relay.go` | Generic `OutboxRelay` struct; `NewKafkaOutboxRelay` / `NewRabbitMQOutboxRelay` constructors; publish failure leaves delivery pending |
| `internal/infrastructure/messaging/rabbitmq_connection.go` | `RabbitMQClient`: dial, exchange declare, `Reconnect()` method |
| `internal/infrastructure/messaging/rabbitmq_publisher.go` | `IEventPublisher` impl — persistent delivery mode; auto-reconnect on error with mutex |
| `internal/infrastructure/messaging/rabbitmq_consumer.go` | Durable queue, routing key binding, prefetch=1, ACK/NACK; auto-reconnect loop with 3s backoff |
| `container/container.go` | Creates **two** `RabbitMQClient` instances (`RabbitMQPublisherClient` + `RabbitMQConsumerClient`); wires `KafkaOutboxRelay` + `RabbitMQOutboxRelay` independently |
| `cmd/worker/cmd.go` | Starts 4 goroutines: kafka relay, rabbitmq relay, kafka consumer, rabbitmq consumer; closes both clients on shutdown |
| `db/migrations/20260524000003_create_outbox_events_table.sql` | Creates `outbox_events` table |
| `db/migrations/20260525000005_create_outbox_deliveries.sql` | Creates `outbox_deliveries` table |

---

## Trade-offs

| | Value |
|---|---|
| Delivery guarantee | At-least-once — relay retries failed delivery rows |
| Latency | ~2s (configurable `relayInterval` in `outbox_relay.go`) |
| Consumer requirement | Must be idempotent — use `event_id` (UUID) to deduplicate if needed |
| RabbitMQ outage | API still works — deliveries queue up in MySQL; publisher auto-reconnects and consumer resumes after broker restarts, no worker restart needed |
| Independent retry | Kafka failure does not block RabbitMQ delivery and vice versa |
| vs direct publish | Direct publish after `tx.Commit()` loses the event on crash; outbox survives |

---

## Running locally

```bash
# 1. Start DB + RabbitMQ + Kafka
docker compose -f docker/docker-compose.yaml up -d db rabbitmq kafka

# 2. Run migrations
go run main.go migrate

# 3. Build binary
go build -o /tmp/app .

# Terminal 1 — HTTP API server
/tmp/app api

# Terminal 2 — worker (both relays + both consumers)
/tmp/app worker
```

RabbitMQ management UI: http://localhost:15672 (guest / guest)

---

## Real test scenarios

### Scenario 1 — Happy path: full flow end-to-end

```bash
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy milk"}'
```

What happens step by step:
```
1. API returns 201 immediately
2. MySQL: outbox_events row + two outbox_deliveries rows (kafka=pending, rabbitmq=pending)
3. (~2s) KafkaOutboxRelay publishes → kafka delivery=published
4. (~2s) RabbitMQOutboxRelay publishes → rabbitmq delivery=published
5. RabbitMQ NotificationConsumer receives message → logs it → ACK
```

Check delivery table after ~3 seconds:
```bash
docker compose -f docker/docker-compose.yaml exec db \
  mysql -uappuser -papppass appdb \
  -e "SELECT e.event_type, d.destination, d.status, d.published_at FROM outbox_events e JOIN outbox_deliveries d ON d.outbox_event_id = e.id ORDER BY e.id, d.destination;" \
  2>/dev/null | grep -v Warning
```

Expected:
```
event_type    destination  status     published_at
todo.created  kafka        published  2026-05-25 ...
todo.created  rabbitmq     published  2026-05-25 ...
```

Worker log expected:
```json
{"msg":"notification task received","to":"admin@example.com","subject":"New Todo Created"}
{"msg":"domain event received","event_type":"todo.created","aggregate_id":"1"}
```

---

### Scenario 2 — Worker goes down: events queue up in MySQL, catch up on restart

```bash
# 1. Stop the worker
pkill -f "/tmp/app worker"

# 2. Create a todo — API still works
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Created while worker is down"}'
# → 201

# 3. Check DB — deliveries are pending, not lost
docker compose -f docker/docker-compose.yaml exec db \
  mysql -uappuser -papppass appdb \
  -e "SELECT destination, status FROM outbox_deliveries ORDER BY id DESC LIMIT 2;" \
  2>/dev/null | grep -v Warning
# kafka=pending, rabbitmq=pending

# 4. Restart the worker
/tmp/app worker

# 5. Within 2s both relays catch up — both deliveries become published
```

---

### Scenario 3 — RabbitMQ goes down: Kafka unaffected, auto-recovers on restart

```bash
# 1. Stop RabbitMQ
docker compose -f docker/docker-compose.yaml stop rabbitmq

# 2. Create a todo — API still returns 201

# 3. KafkaOutboxRelay publishes its delivery successfully (independent)
#    RabbitMQOutboxRelay: Publish() fails → tries client.Reconnect() → also fails
#    → logs warning, rabbitmq delivery stays pending (NOT marked failed)
#    → consumer goroutine detects !ok → enters reconnect loop (3s backoff)

# 4. Restart RabbitMQ
docker compose -f docker/docker-compose.yaml start rabbitmq

# 5. rabbitmq_publisher: next relay tick → Publish() fails → Reconnect() succeeds
#    → retries Publish() → succeeds → MarkDeliveryPublished()
#    consumer goroutine: Reconnect() succeeds → re-declares queue → resumes consuming
#    → notification delivered — no worker restart required
```

---

## Run unit tests

```bash
go test ./internal/usecase/...
# ok  github.com/yourname/go-clean-base/internal/usecase
```

The usecase tests use `OutboxRepositoryMock` — no real DB or RabbitMQ needed.

---

## Verify architecture rules

```bash
grep -r "infrastructure" internal/domain/    # must be zero results
grep -r "infrastructure" internal/usecase/   # must be zero results
grep -r "amqp" internal/usecase/             # must be zero results
```

- `IOutboxRepository` lives in `domain/repository/` — not in infrastructure
- `IEventPublisher` lives in `domain/service/` — not in infrastructure
- `amqp091-go` import only in `internal/infrastructure/messaging/`
- Usecase depends only on domain interfaces — completely decoupled from RabbitMQ
