package messaging

import (
	"context"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// KafkaConsumer reads domain events from a Kafka topic using manual offset commit.
//
// Why manual commit instead of auto-commit?
// Auto-commit advances the offset on a timer, regardless of whether the handler
// finished. If the process crashes after auto-commit but before the handler
// completes, the message is lost. With manual commit the offset only advances
// after a successful handler return — guaranteeing at-least-once delivery.
//
// Trade-off: if the process crashes after a successful handler but before
// CommitMessages, the message is redelivered on restart. Handlers should
// therefore be idempotent (safe to process the same event twice).
type KafkaConsumer struct {
	// reader is a long-lived kafka.Reader bound to a consumer group.
	// It tracks partition assignment and offset management automatically.
	reader *kafka.Reader
}

// NewKafkaConsumer wraps a pre-configured kafka.Reader. Called once at startup.
func NewKafkaConsumer(reader *kafka.Reader) *KafkaConsumer {
	return &KafkaConsumer{reader: reader}
}

// Start enters a blocking read loop. For each message it:
//  1. Fetches the next message from the assigned partition (blocks until available).
//  2. Calls handler with the raw message body.
//  3. Commits the offset only if the handler succeeded.
//
// If handler returns an error the offset is NOT committed — the message will be
// redelivered on the next call (or after a worker restart).
//
// The loop exits cleanly when ctx is cancelled (SIGINT / SIGTERM from errgroup).
func (c *KafkaConsumer) Start(ctx context.Context, handler func([]byte) error) error {
	for {
		// FetchMessage does NOT auto-commit — commit only after successful processing.
		// Blocks until a message is available or ctx is cancelled.
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // graceful shutdown — context cancelled, stop the loop
			}
			slog.ErrorContext(ctx, "kafka fetch failed", "error", err)
			continue
		}

		if err := handler(msg.Value); err != nil {
			// Do not commit — broker will redeliver this message on next poll.
			slog.ErrorContext(ctx, "kafka handler failed — not committing",
				"error", err, "offset", msg.Offset, "partition", msg.Partition)
			continue
		}

		// Commit advances the consumer group offset for this partition.
		// After a worker restart, reading resumes from the next uncommitted offset.
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			slog.ErrorContext(ctx, "kafka commit failed", "error", err)
		}
	}
}

// Close commits any pending offsets and closes the connection to the broker.
// Called from the worker defer block on graceful shutdown.
func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}
