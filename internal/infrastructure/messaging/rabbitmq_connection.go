package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yourname/go-clean-base/config"
)

// RabbitMQClient holds the single AMQP TCP connection and a shared channel.
//
// Connection vs Channel:
//   - Connection: one TCP socket to the broker. Expensive to create; reused for
//     the entire application lifetime.
//   - Channel: a lightweight virtual connection multiplexed over the TCP socket.
//     Publishing and consuming both use this shared channel. For high-throughput
//     scenarios you can open multiple channels per connection.
//
// Both publisher and consumer share this client so they reuse the same TCP
// connection, which is the recommended pattern for low-connection-count setups.
type RabbitMQClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	cfg     *config.MessagingConfig
}

// NewRabbitMQClient dials the AMQP broker, opens a channel, and declares the
// topic exchange. Called once at startup from the container.
//
// Exchange declaration is idempotent — safe to call on every startup.
// If the exchange already exists with the same type and durability, it's a no-op.
// If it exists with different parameters, RabbitMQ returns an error (prevents
// misconfiguration from silently changing a live exchange).
//
// Exchange type "topic" allows routing by pattern:
//   - "todo.created" matches binding key "todo.*" or "todo.#" or "#"
//   - Consumers bind queues with patterns to subscribe to specific event subsets
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

	// Declare the topic exchange. durable=true: exchange survives broker restart.
	// auto-delete=false: keep the exchange even when no queues are bound.
	// internal=false: clients can publish directly to this exchange.
	if err := ch.ExchangeDeclare(
		cfg.RabbitMQExchange,
		"topic", // routes by routing key wildcard patterns
		true,    // durable: survives broker restart
		false,   // auto-delete: keep even with no bindings
		false,   // internal: allow external publishers
		false,   // no-wait: wait for broker confirmation
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq declare exchange: %w", err)
	}

	return &RabbitMQClient{conn: conn, channel: ch, cfg: cfg}, nil
}

// Channel returns the shared AMQP channel. Used by publishers and consumers
// that need direct channel access (e.g. notificationPublisher).
func (c *RabbitMQClient) Channel() *amqp.Channel {
	return c.channel
}

// Close shuts down the channel first, then the TCP connection.
// Closing the channel flushes any pending confirms; closing the connection
// terminates all multiplexed channels. Called from the worker defer block.
func (c *RabbitMQClient) Close() error {
	if err := c.channel.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}
