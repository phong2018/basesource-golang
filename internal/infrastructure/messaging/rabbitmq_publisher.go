package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

type rabbitmqPublisher struct {
	mu     sync.Mutex
	client *RabbitMQClient
}

// NewRabbitMQPublisher returns a RabbitMQ-backed IEventPublisher.
// The returned publisher writes to the topic exchange declared in RabbitMQClient.
// It reconnects automatically when the broker connection drops and comes back.
func NewRabbitMQPublisher(client *RabbitMQClient) domainService.IEventPublisher {
	return &rabbitmqPublisher{client: client}
}

// Publish serialises the OutboxEvent and publishes it to the topic exchange.
// On connection failure it attempts one reconnect before returning the error.
func (p *rabbitmqPublisher) Publish(ctx context.Context, event *domainModel.OutboxEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.doPublish(ctx, event); err == nil {
		return nil
	}
	slog.WarnContext(ctx, "rabbitmq publish failed, attempting reconnect", "event_id", event.EventID)
	if err := p.client.Reconnect(); err != nil {
		return fmt.Errorf("rabbitmq reconnect: %w", err)
	}
	return p.doPublish(ctx, event)
}

func (p *rabbitmqPublisher) doPublish(ctx context.Context, event *domainModel.OutboxEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return p.client.channel.PublishWithContext(ctx,
		p.client.cfg.RabbitMQExchange,
		event.EventType,
		false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    event.EventID,
			Body:         body,
		},
	)
}

func (p *rabbitmqPublisher) Close() error { return nil }
