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
    INSERT todos            ┐
    INSERT audit_logs       ├ one atomic transaction
    INSERT outbox_events    ┘  status=pending

(process can crash here — row is already in MySQL)

OutboxRelay wakes up every 2s:
    SELECT pending rows → publish to RabbitMQ → mark published
```

---

## Architecture

```
[API server process]                 [Worker process]
─────────────────────                ──────────────────────────────────────
POST /todos                          goroutine 1: OutboxRelay (publisher)
  └─ usecase.Create()                  └─ every 2s:
       └─ WithinTransaction()               SELECT outbox_events WHERE status=pending
            ├─ INSERT todos                 publisher.Publish(event) → RabbitMQ
            ├─ INSERT audit_logs            UPDATE outbox_events SET status=published
            └─ INSERT outbox_events
               status=pending        goroutine 2: RabbitMQConsumer
                                       └─ queue: todo.events.worker
                    ↕ MySQL                 handler(body) → log / process event
                                           Ack() on success
                    ↕ RabbitMQ             Nack() → dead-letter on failure
```

**Key rule:** the usecase writes only to MySQL. It never calls RabbitMQ directly. The relay worker is the only component that talks to the broker.

---

## Publisher vs Consumer

### Publisher — the outbox relay

The **publisher** is `OutboxRelay` running inside `cmd/worker`.

- Polls `outbox_events WHERE status='pending'` every 2 seconds
- Calls `rabbitmq_publisher.Publish(event)` → sends to exchange `todo.events` with routing key = `event.EventType` (e.g. `todo.created`)
- On success: marks row `published`
- On failure: marks row `failed`, logs the error

```
usecase.Create()
    INSERT outbox_events (status=pending)   ← usecase writes here, NOT to RabbitMQ

OutboxRelay (2s later)
    SELECT status=pending
    → rabbitmq_publisher.Publish()          ← THIS is the publisher
    → UPDATE status=published
```

### Consumer — the event handler

The **consumer** is `RabbitMQConsumer` running inside `cmd/worker`.

- Declares durable queue `todo.events.worker`, binds to exchange `todo.events` with routing key `#` (all events)
- `prefetch=1` — processes one message at a time (fair dispatch)
- Calls `handler(body)` for each message
- Sends `Ack()` when handler succeeds → RabbitMQ removes the message
- Sends `Nack(requeue=false)` when handler fails → message goes to dead-letter queue

```
RabbitMQ exchange: todo.events
    │ routing key: todo.created / todo.updated / todo.deleted
    ▼
Queue: todo.events.worker
    │
    ▼
RabbitMQConsumer
    → handler(body) → log event (in real system: send email, update search index, etc.)
    → Ack()
```

### How they relate

```
[API server]          [MySQL]          [RabbitMQ]          [Worker]
     │                   │                  │                  │
usecase.Create()         │                  │                  │
  INSERT outbox_events ──▶                  │                  │
                         │                  │            OutboxRelay polls
                         ◀── SELECT pending │            every 2s
                         │                  │                  │
                         │           Publish(event) ──────────▶
                         │                  │                  │
                  mark published             │           Consumer receives
                         │                  ◀────────────────── │
                         │                  │             Ack() │
```

The publisher and consumer never talk to each other directly — only through RabbitMQ.

---

## New files

| File | Purpose |
|---|---|
| `cmd/worker/cmd.go` | `go run main.go worker` — starts relay + consumer with graceful shutdown via errgroup |
| `internal/infrastructure/messaging/rabbitmq_connection.go` | Shared RabbitMQ connection + topic exchange declaration |
| `internal/infrastructure/messaging/rabbitmq_publisher.go` | Implements `IEventPublisher` — publishes with persistent delivery mode |
| `internal/infrastructure/messaging/rabbitmq_consumer.go` | Declares durable queue, binds routing key, prefetch=1, ACK/NACK |
| `internal/infrastructure/messaging/outbox_relay.go` | Polls `outbox_events` every 2s, publishes, marks published/failed |
| `internal/infrastructure/repository/outbox_repository_impl.go` | DB CRUD for `outbox_events` table |
| `internal/domain/model/event.go` | `OutboxEvent` entity + event type/status constants |
| `internal/domain/repository/outbox_repository.go` | `IOutboxRepository` interface |
| `internal/domain/service/event_publisher.go` | `IEventPublisher` interface |
| `internal/domain/repository/mock/outbox_repository_mock.go` | Mock for unit tests |
| `db/migrations/20260524000003_create_outbox_events_table.sql` | Creates `outbox_events` table |

Modified: `config/config.go`, `.env.example`, `docker/docker-compose.yaml`, `todo_usecase_impl.go`, `todo_usecase_impl_test.go`, `container/container.go`, `main.go`

---

## Trade-offs

| | Value |
|---|---|
| Delivery guarantee | At-least-once — relay retries failed rows |
| Latency | ~2s (configurable `relayInterval` in `outbox_relay.go`) |
| Consumer requirement | Must be idempotent — use `event_id` (UUID) to deduplicate if needed |
| RabbitMQ outage | API still works — events queue up in MySQL, catch up on recovery |
| vs Kafka | RabbitMQ: push-based, message deleted after ACK. Kafka: pull-based, messages retained for replay |

---

## Running locally

```bash
# 1. Add RabbitMQ vars to .env
echo "RABBITMQ_URL=amqp://guest:guest@localhost:5672/" >> .env
echo "RABBITMQ_EXCHANGE=todo.events" >> .env

# 2. Start DB + RabbitMQ
docker compose -f docker/docker-compose.yaml up -d db rabbitmq

# 3. Run migrations (creates outbox_events table)
go run main.go migrate

# 4. Build binary (ensures both processes use same compiled code)
go build -o /tmp/app .

# Terminal 1 — HTTP API server
/tmp/app api

# Terminal 2 — worker (relay + consumer)
/tmp/app worker
```

RabbitMQ management UI: http://localhost:15672 (guest / guest)

---

## Real test scenarios

### Scenario 1 — Happy path: full flow end-to-end

**Verify:** API → MySQL outbox → relay publishes → consumer receives.

```bash
# Create a todo
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy milk"}'
```

What happens step by step:
```
1. API returns 201 immediately
2. In MySQL: outbox_events row with status=pending
3. (2s later) relay wakes up → publishes to RabbitMQ → status=published
4. Consumer receives message → logs it → ACK
```

Check outbox table after ~3 seconds:
```bash
docker compose -f docker/docker-compose.yaml exec db \
  mysql -uappuser -papppass appdb \
  -e "SELECT event_type, aggregate_id, status, published_at FROM outbox_events ORDER BY id;" \
  2>/dev/null | grep -v Warning
```

Expected:
```
event_type    aggregate_id  status     published_at
todo.created  1             published  2026-05-24 ...
```

Worker log expected:
```json
{"msg":"event received","event_type":"todo.created","aggregate_id":"1"}
```

---

### Scenario 2 — All three event types (Create / Update / Delete)

**Verify:** each operation produces the correct event type.

```bash
# Create
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Test event types"}'
# note the id returned, e.g. 5

# Update
curl -s -X PUT http://localhost:8080/api/v1/todos/5 \
  -H "Content-Type: application/json" \
  -d '{"title":"Updated","done":true}'

# Delete
curl -s -X DELETE http://localhost:8080/api/v1/todos/5
```

Worker log expected (in order):
```json
{"msg":"event received","event_type":"todo.created","aggregate_id":"5"}
{"msg":"event received","event_type":"todo.updated","aggregate_id":"5"}
{"msg":"event received","event_type":"todo.deleted","aggregate_id":"5"}
```

DB expected:
```
event_type    status
todo.created  published
todo.updated  published
todo.deleted  published
```

---

### Scenario 3 — Worker goes down: events queue up in MySQL, catch up on restart

**Verify:** publisher crash does not lose events.

```bash
# 1. Stop the worker (Ctrl+C in Terminal 2)

# 2. Create a todo — API still works
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Created while worker is down"}'
# → 201 ← API unaffected

# 3. Check DB — event is pending, not lost
docker compose -f docker/docker-compose.yaml exec db \
  mysql -uappuser -papppass appdb \
  -e "SELECT event_type, status FROM outbox_events ORDER BY id DESC LIMIT 1;" \
  2>/dev/null | grep -v Warning
# status: pending ← safely stored

# 4. Restart the worker
/tmp/app worker

# 5. Within 2s relay catches up — check DB again
# status: published ← relay published the queued event
```

**What this proves:** the outbox pattern gives durable at-least-once delivery. Direct `broker.Publish()` after `tx.Commit()` would permanently lose the event if the process crashes in between.

---

### Scenario 4 — RabbitMQ goes down: API still works, events catch up on recovery

**Verify:** RabbitMQ outage does not affect the API or data integrity.

```bash
# 1. Stop RabbitMQ
docker compose -f docker/docker-compose.yaml stop rabbitmq

# 2. Create a todo — API still returns 201
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Created during RabbitMQ outage"}'
# → 201 ← MySQL transaction committed fine

# 3. Worker logs relay errors but keeps retrying every 2s
# {"msg":"outbox relay: list pending failed","error":"...connection refused"}

# 4. Restart RabbitMQ
docker compose -f docker/docker-compose.yaml start rabbitmq

# 5. Within 2s relay publishes all pending rows
# {"msg":"event received","event_type":"todo.created","aggregate_id":"..."}
```

**What this proves:** the API and outbox table are completely isolated from RabbitMQ availability. Users never see errors during broker outages. Compare to direct HTTP call to a notification service — that would return 500 to the user during an outage.

---

### Scenario 5 — Multiple worker instances share the load (consumer group)

**Verify:** horizontal scaling — each message is processed by exactly one worker.

```bash
# Start 3 worker instances (each connects to the same queue)
/tmp/app worker   # Terminal 2
/tmp/app worker   # Terminal 3
/tmp/app worker   # Terminal 4

# Create 9 todos rapidly
for i in $(seq 1 9); do
  curl -s -X POST http://localhost:8080/api/v1/todos \
    -H "Content-Type: application/json" \
    -d "{\"title\":\"Todo $i\"}" &
done
wait
```

Expected: 9 events distributed across 3 terminals (~3 each). Each event appears in exactly one terminal — not all three.

```
Terminal 2: event received aggregate_id=1,4,7
Terminal 3: event received aggregate_id=2,5,8
Terminal 4: event received aggregate_id=3,6,9
```

**What this proves:** RabbitMQ round-robins messages across consumers connected to the same queue. Adding more workers scales throughput — no code change needed anywhere.

---

## Run unit tests

```bash
go test ./internal/usecase/...
# ok  github.com/yourname/go-clean-base/internal/usecase
```

The usecase tests use `OutboxRepositoryMock` — no real DB or RabbitMQ needed. The mock is in `internal/domain/repository/mock/outbox_repository_mock.go`.

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
