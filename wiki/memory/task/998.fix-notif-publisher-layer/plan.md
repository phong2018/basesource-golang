# Plan: Move `PublishNotification` from Usecase → Worker Event Handler

## Problem
`todoUsecase.Create()` calls `PublishNotification` (RabbitMQ) directly after the DB transaction.
This violates the architecture rule: **usecase must never call RabbitMQ directly**.

## Correct Flow (after fix)
```
API: POST /todos → usecase.Create()
  → DB transaction only (todos + audit_logs + outbox_events status=pending)
  [no RabbitMQ]

Worker: OutboxRelay (every 2s)
  → SELECT pending outbox_events → publish to Kafka → mark published

Worker: KafkaConsumer
  → HandleDomainEvent.Handle(body)
  → build Notification from OutboxEvent
  → notifPublisher.PublishNotification()  ← correct place

Worker: RabbitMQConsumer
  → HandleNotificationTask → log → Ack()
```

---

## Steps

### Step 1 — `domain_event_handler.go`: plain func → struct with injected notifPublisher
**File:** `internal/presentation/worker/domain_event_handler.go`
- Replace `func HandleDomainEvent(body []byte) error` with struct `DomainEventHandler`
- Add `notifPublisher domainSvc.INotificationPublisher` field
- Constructor: `func NewDomainEventHandler(p domainSvc.INotificationPublisher) *DomainEventHandler`
- Method: `func (h *DomainEventHandler) Handle(body []byte) error`
- Inside `Handle`: unmarshal OutboxEvent, build Notification, call `h.notifPublisher.PublishNotification`

### Step 2 — `container.go`: rewire notifPublisher from usecase → DomainEventHandler
**File:** `container/container.go`
- Add `DomainEventHandler *workerPresentation.DomainEventHandler` to Container struct
- Construct: `domainEventHandler := workerPresentation.NewDomainEventHandler(rmqNotifPublisher)`
- Pass `domainEventHandler.Handle` to KafkaConsumer (not the old free function)
- Remove `rmqNotifPublisher` from `usecase.NewTodoUsecase(...)` call

### Step 3 — `todo_usecase_impl.go`: remove notifPublisher
**File:** `internal/usecase/todo_usecase_impl.go`
- Remove `notifPublisher domainSvc.INotificationPublisher` field from struct
- Remove parameter from `NewTodoUsecase`
- Delete post-transaction block in `Create()` (Notification literal + PublishNotification call)

### Step 4 — `todo_usecase_impl_test.go`: remove mock
**File:** `internal/usecase/todo_usecase_impl_test.go`
- Remove `&serviceMock.NotificationPublisherMock{}` from `NewTodoUsecase(...)` call
- Remove `serviceMock` import if no longer used

### Step 5 — `test_05_durability.sh`: rewrite durability proof
**File:** `wiki/memory/task/994.script-test-rabbitmq/test_05_durability.sh`

New scenario: "worker offline → POST todo → outbox_event pending in DB (not in RabbitMQ) → restart worker → relay+kafka → notification delivered"

- Step 3 (POST while offline): assert `outbox_events status=pending` in DB (not RabbitMQ queue depth)
- Step 4: assert RabbitMQ queue depth = 0 (nothing published yet — worker offline)
- Step 5 (restart): wait for relay + kafka consumer startup logs
- Steps 6/7: notification delivered + queue empty (unchanged)

### Step 6 — `test_06_queue_while_offline.sh`: same rewrite as test_05 for batch
**File:** `wiki/memory/task/994.script-test-rabbitmq/test_06_queue_while_offline.sh`

- After batch POST: assert `COUNT(*) FROM outbox_events WHERE status='pending' = BATCH` in DB
- Assert RabbitMQ queue depth = 0 (not BATCH) before restart
- After restart: all BATCH notifications delivered + queue empty

---

## Files NOT changing
- `internal/domain/service/notification_publisher.go` — interface stays
- `test_01_basic_notification.sh` — happy path still works (slightly slower: ~2s relay delay)
- `test_07_nack.sh` — consumer behavior, unaffected
- `test_08_competing_consumers.sh` — consumer behavior, unaffected

---

## Dependency rule after fix
| Layer | Imports |
|---|---|
| `usecase` | `domain/repository`, `domain/service` only — no RabbitMQ |
| `presentation/worker` | `domain/service` (allowed) — holds notifPublisher |
| `container` | wires everything at startup |

```bash
grep -r "amqp" internal/usecase/   # must be zero results after fix
```
