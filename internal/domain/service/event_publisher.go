package service

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

// IEventPublisher is the domain-layer port for publishing domain events.
// It is broker-agnostic — the domain layer has no knowledge of whether the
// underlying transport is Kafka, RabbitMQ, or anything else.
//
// Two infrastructure implementations exist:
//   - kafkaProducer  (messaging/kafka_producer.go)  — writes to a Kafka topic;
//     used by OutboxRelay in feature/996 for durable, replayable event streaming.
//   - publisher      (messaging/rabbitmq_publisher.go) — publishes to a RabbitMQ
//     topic exchange; used by OutboxRelay in feature/997 as an alternative transport.
//
// Which implementation is injected is decided at startup in container/container.go.
// Swapping brokers requires no changes to the domain or usecase layers.
//
// Used exclusively by OutboxRelay — the relay polls pending outbox_events from
// the DB and calls Publish() to forward them to the broker. The usecase layer
// never calls this directly; it only writes to the outbox table inside a DB
// transaction, letting the relay handle the async fan-out.
type IEventPublisher interface {
	// Publish sends a single OutboxEvent to the configured broker.
	// Implementations must be synchronous — the caller (OutboxRelay) relies on
	// the error return to decide whether to mark the outbox row as published.
	Publish(ctx context.Context, event *model.OutboxEvent) error

	// Close flushes any buffered messages and releases the underlying connection.
	Close() error
}
