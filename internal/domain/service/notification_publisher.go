package service

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

// INotificationPublisher is the domain-layer port for sending notification tasks
// to an async task queue (RabbitMQ).
//
// Why a separate interface from IEventPublisher?
//   - IEventPublisher carries domain OutboxEvents to a durable event stream
//     (current impl: Kafka) — used by OutboxRelay for audit log / event sourcing.
//   - INotificationPublisher carries Notification tasks to a task queue
//     (current impl: RabbitMQ) — used by the usecase layer for fire-and-forget jobs.
//
// The usecase layer depends only on this interface — it knows nothing about
// RabbitMQ, channels, or AMQP. The infrastructure adapter (notificationPublisher)
// is injected at startup via the container.
type INotificationPublisher interface {
	// PublishNotification enqueues a notification task (e.g. send email, push alert).
	// The message is persisted on the broker — delivery is guaranteed even if the
	// consumer is temporarily offline.
	PublishNotification(ctx context.Context, n *model.Notification) error

	// Close releases the underlying broker connection / channel.
	Close() error
}
