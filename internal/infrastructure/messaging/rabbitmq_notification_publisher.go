package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

// rabbitmqNotificationPublisher implements INotificationPublisher using RabbitMQ.
// It is the RabbitMQ side of the dual-broker setup: notification tasks
// (e.g. "send email to user") are enqueued here by the usecase layer and
// consumed asynchronously by the NotificationConsumer goroutine in the worker.
//
// Why RabbitMQ for notifications instead of Kafka?
//   - Notifications are one-time tasks: each message must be processed by
//     exactly one consumer and then removed. RabbitMQ queues model this
//     naturally (message is ACK'd and deleted after processing).
//   - No need for replay or long-term retention — if a notification was
//     already sent, replaying it would send duplicates.
type rabbitmqNotificationPublisher struct {
	// ch is a pre-opened AMQP channel shared from RabbitMQClient.
	// A channel is a lightweight virtual connection multiplexed over the
	// single TCP connection; publishing is thread-safe through this channel.
	ch *amqp.Channel
}

// NewRabbitMQNotificationPublisher returns a rabbitmqNotificationPublisher backed by the
// shared RabbitMQClient channel. Called once at startup from the container.
func NewRabbitMQNotificationPublisher(client *RabbitMQClient) domainService.INotificationPublisher {
	return &rabbitmqNotificationPublisher{ch: client.channel}
}

// PublishNotification serialises the Notification to JSON and publishes it to
// the RabbitMQ default exchange with the queue name as the routing key.
//
// Publishing to the default exchange ("") with routingKey = queue name is
// equivalent to writing directly to the queue — no explicit binding needed.
//
// DeliveryMode: Persistent — the broker writes the message to disk so it
// survives a broker restart. If the worker is offline when a notification
// is published, the message waits in the queue until the consumer reconnects.
func (p *rabbitmqNotificationPublisher) PublishNotification(ctx context.Context, n *domainModel.Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	return p.ch.PublishWithContext(ctx,
		"",                   // default exchange — routes directly to the named queue
		"todo.notifications", // routing key = queue name
		false,                // mandatory: false — drop silently if no queue is bound
		false,                // immediate: false — not supported in RabbitMQ 3.x+
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // survives broker restart
			Body:         body,
		},
	)
}

// Close is a no-op — channel lifecycle is managed by RabbitMQClient.Close().
func (p *rabbitmqNotificationPublisher) Close() error { return nil }
