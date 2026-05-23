package messaging

import (
	"context"
	"fmt"
	"log/slog"
)

type RabbitMQConsumer struct {
	client *RabbitMQClient
}

func NewRabbitMQConsumer(client *RabbitMQClient) *RabbitMQConsumer {
	return &RabbitMQConsumer{client: client}
}

func (c *RabbitMQConsumer) Start(ctx context.Context, queueName string, routingKey string, handler func([]byte) error) error {
	ch := c.client.channel

	if err := ch.Qos(c.client.cfg.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	q, err := ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare queue %s: %w", queueName, err)
	}

	if err := ch.QueueBind(q.Name, routingKey, c.client.cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", queueName, err)
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", queueName, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-msgs:
				if !ok {
					return
				}
				if err := handler(d.Body); err != nil {
					slog.ErrorContext(ctx, "consumer handler failed, nacking", "queue", queueName, "error", err)
					_ = d.Nack(false, false) // dead-letter
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}()

	return nil
}
