package messaging

import (
	"context"
	"fmt"
	"log/slog"
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

	// Internal goroutine — runs for the lifetime of the worker.
	go func() {
		for {
			select {
			case <-ctx.Done():
				// Graceful shutdown: stop consuming when context is cancelled.
				return
			case d, ok := <-msgs:
				if !ok {
					// Channel closed by broker (e.g. connection lost). Exit loop.
					return
				}
				if err := handler(d.Body); err != nil {
					slog.ErrorContext(ctx, "consumer handler failed, nacking", "queue", queueName, "error", err)
					// NACK without requeue — avoids infinite retry loop on poison messages.
					// Configure a dead-letter exchange on the queue to capture failed messages.
					_ = d.Nack(false, false)
				} else {
					// ACK — broker removes the message from the queue permanently.
					_ = d.Ack(false)
				}
			}
		}
	}()

	return nil
}
