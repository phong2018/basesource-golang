package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yourname/go-clean-base/config"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

// rabbitmqPublisher implements IEventPublisher using RabbitMQ.
// It publishes domain OutboxEvents to a topic exchange so multiple queues
// can bind with different routing key patterns and each receive their own copy.
//
// Note: in feature/996 this publisher is superseded by kafkaProducer for the
// outbox relay path (domain events now go to Kafka). This file is retained as
// an alternative IEventPublisher implementation — useful if you want to fan-out
// domain events to RabbitMQ consumers in addition to, or instead of, Kafka.
type rabbitmqPublisher struct {
	// ch is the AMQP channel used for publishing.
	ch  *amqp.Channel
	// cfg holds exchange name and other messaging settings.
	cfg *config.MessagingConfig
}

// NewRabbitMQPublisher returns a RabbitMQ-backed IEventPublisher.
// The returned publisher writes to the topic exchange declared in RabbitMQClient.
func NewRabbitMQPublisher(client *RabbitMQClient) domainService.IEventPublisher {
	return &rabbitmqPublisher{ch: client.channel, cfg: client.cfg}
}

// Publish serialises the OutboxEvent to JSON and publishes it to the topic exchange.
//
// Routing key = EventType (e.g. "todo.created", "todo.deleted").
// Consumers bind queues to the exchange with patterns like "todo.*" or "#"
// to receive the events they care about.
//
// MessageId = EventID provides deduplication: consumers can use this field
// to detect and skip redelivered messages (idempotency).
//
// DeliveryMode: Persistent — message survives broker restart.
func (p *rabbitmqPublisher) Publish(ctx context.Context, event *domainModel.OutboxEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.ch.PublishWithContext(ctx,
		p.cfg.RabbitMQExchange, // topic exchange — routes by EventType pattern
		event.EventType,        // routing key — e.g. "todo.created"
		false,                  // mandatory: false — drop if no queue is bound
		false,                  // immediate: false — not supported in RabbitMQ 3.x+
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // survives broker restart
			MessageId:    event.EventID,   // used for consumer-side deduplication
			Body:         body,
		},
	)
}

// Close is a no-op — channel lifecycle is managed by RabbitMQClient.Close().
func (p *rabbitmqPublisher) Close() error {
	return nil
}
