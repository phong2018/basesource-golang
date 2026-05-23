package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yourname/go-clean-base/config"
)

type RabbitMQClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	cfg     *config.MessagingConfig
}

func NewRabbitMQClient(cfg *config.MessagingConfig) (*RabbitMQClient, error) {
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq open channel: %w", err)
	}

	if err := ch.ExchangeDeclare(
		cfg.Exchange,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq declare exchange: %w", err)
	}

	return &RabbitMQClient{conn: conn, channel: ch, cfg: cfg}, nil
}

func (c *RabbitMQClient) Channel() *amqp.Channel {
	return c.channel
}

func (c *RabbitMQClient) Close() error {
	if err := c.channel.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}
