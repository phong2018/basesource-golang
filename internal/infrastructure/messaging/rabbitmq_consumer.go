package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConsumer subscribes to a RabbitMQ queue and dispatches each message
// to a caller-supplied handler function.
//
// It is used for the notification task queue: the worker goroutine calls Start()
// once, which sets up the queue/binding and launches an internal goroutine that
// reads messages in a loop. The caller's goroutine is freed immediately — the
// worker uses <-ctx.Done() to wait for shutdown.
type RabbitMQConsumer struct {
	client *RabbitMQClient
}

// NewRabbitMQConsumer wraps the shared RabbitMQClient. Called once at startup.
func NewRabbitMQConsumer(client *RabbitMQClient) *RabbitMQConsumer {
	return &RabbitMQConsumer{client: client}
}

// Start declares the queue, binds it to the exchange, and begins consuming.
// It returns immediately after launching an internal goroutine — it does NOT block.
//
// Parameters:
//   - queueName:  name of the durable queue to declare and consume (e.g. "todo.notifications")
//   - routingKey: binding key on the exchange (for the default exchange, equals queueName)
//   - handler:    called with the raw message body; return nil to ACK, error to NACK
//
// Queue is declared durable (survives broker restart) and non-exclusive (shared
// across multiple worker instances for competing-consumer load distribution).
//
// Prefetch (QoS):
// RabbitMQPrefetchCount limits how many unacknowledged messages the broker
// pushes to this consumer at once. Setting it to 1 ensures fair dispatch —
// a slow consumer won't hoard messages while others are idle.
//
// ACK / NACK:
//   - handler success → d.Ack(false) — message removed from queue permanently
//   - handler error   → d.Nack(false, false) — message is NOT requeued; if a
//     dead-letter exchange is configured it receives the message, otherwise it
//     is dropped. This prevents a poison message from looping forever.
func (c *RabbitMQConsumer) Start(ctx context.Context, queueName string, routingKey string, handler func([]byte) error) error {
	ch := c.client.channel

	// Prefetch count: broker sends at most N unacknowledged messages at a time.
	// 0 for prefetchSize (byte limit, 0 = unlimited), false = per-consumer not per-channel.
	if err := ch.Qos(c.client.cfg.RabbitMQPrefetchCount, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	// QueueDeclare is idempotent — safe to call on every startup.
	// If the queue already exists with the same parameters, this is a no-op.
	q, err := ch.QueueDeclare(
		queueName,
		true,  // durable: queue survives broker restart
		false, // auto-delete: keep queue even when no consumers are connected
		false, // exclusive: allow multiple consumers (competing workers)
		false, // no-wait: wait for broker confirmation
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare queue %s: %w", queueName, err)
	}

	// Bind the queue to the exchange with the given routing key.
	// Messages published to the exchange with a matching key are routed here.
	if err := ch.QueueBind(q.Name, routingKey, c.client.cfg.RabbitMQExchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", queueName, err)
	}

	// Consume returns a Go channel that receives deliveries as they arrive.
	// autoAck=false: we manually ACK/NACK after processing (safe delivery).
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", queueName, err)
	}

	// setup declares the queue, binds it, and returns a new delivery channel.
	setup := func() (<-chan amqp.Delivery, error) {
		ch := c.client.channel
		if err := ch.Qos(c.client.cfg.RabbitMQPrefetchCount, 0, false); err != nil {
			return nil, fmt.Errorf("set qos: %w", err)
		}
		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		if err != nil {
			return nil, fmt.Errorf("declare queue %s: %w", queueName, err)
		}
		if err := ch.QueueBind(q.Name, routingKey, c.client.cfg.RabbitMQExchange, false, nil); err != nil {
			return nil, fmt.Errorf("bind queue %s: %w", queueName, err)
		}
		return ch.Consume(q.Name, "", false, false, false, false, nil)
	}

	// Internal goroutine — runs for the lifetime of the worker.
	// Reconnects automatically when the broker connection drops and recovers.
	go func() {
		deliveries := msgs
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					// Channel closed by broker (connection lost). Reconnect with backoff.
					slog.WarnContext(ctx, "rabbitmq consumer channel closed, reconnecting", "queue", queueName)
					for {
						select {
						case <-ctx.Done():
							return
						case <-time.After(3 * time.Second):
						}
						if err := c.client.Reconnect(); err != nil {
							slog.ErrorContext(ctx, "rabbitmq consumer reconnect failed", "queue", queueName, "error", err)
							continue
						}
						var err error
						deliveries, err = setup()
						if err != nil {
							slog.ErrorContext(ctx, "rabbitmq consumer re-setup failed", "queue", queueName, "error", err)
							continue
						}
						slog.InfoContext(ctx, "rabbitmq consumer reconnected", "queue", queueName)
						break
					}
					continue
				}
				if err := handler(d.Body); err != nil {
					slog.ErrorContext(ctx, "consumer handler failed, nacking", "queue", queueName, "error", err)
					_ = d.Nack(false, false)
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}()

	return nil
}
