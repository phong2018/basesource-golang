# Branch Features

Each branch demonstrates a specific production pattern layered on top of the Clean Architecture base.

| Branch | Feature | Detail |
|---|---|---|
| `master` | Base: Todo CRUD, audit log, S3, HTTP notification, transaction, pagination, lint, unit tests | — |
| `feature/997` | RabbitMQ task queue + Outbox pattern | [997-rabbitmq-outbox.md](997-rabbitmq-outbox.md) |
| `feature/996` | Dual relay: Kafka + RabbitMQ both driven by outbox pattern (extends feature/997) | [996-kafka-outbox.md](996-kafka-outbox.md) |
