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

type publisher struct {
	ch  *amqp.Channel
	cfg *config.MessagingConfig
}

func NewPublisher(client *RabbitMQClient) domainService.IEventPublisher {
	return &publisher{ch: client.channel, cfg: client.cfg}
}

func (p *publisher) Publish(ctx context.Context, event *domainModel.OutboxEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.ch.PublishWithContext(ctx,
		p.cfg.Exchange,  // exchange
		event.EventType, // routing key
		false,           // mandatory
		false,           // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    event.EventID,
			Body:         body,
		},
	)
}

func (p *publisher) Close() error {
	return nil // channel lifecycle managed by RabbitMQClient
}
