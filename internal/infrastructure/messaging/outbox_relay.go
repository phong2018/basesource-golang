package messaging

import (
	"context"
	"log/slog"
	"time"

	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

// relayBatchSize caps how many pending events are processed per tick.
// Prevents a large backlog from causing a single tick to block for too long.
const (
	relayBatchSize = 10
	relayInterval  = 2 * time.Second
)

// OutboxRelay is the background worker that implements the Outbox Pattern.
//
// The outbox pattern solves the dual-write problem:
//
//	Problem: writing to the DB and publishing to a message broker in the same
//	         operation is not atomic. If the broker publish fails after the DB
//	         commit, the event is lost. If the DB commit fails after a publish,
//	         a ghost event exists in the broker.
//
//	Solution: the usecase writes the event to an outbox_events table inside the
//	          same DB transaction as the business data. The OutboxRelay polls the
//	          outbox table and publishes events to the broker asynchronously.
//	          The DB commit is the single source of truth — the relay retries
//	          until publish succeeds, so no event is ever permanently lost.
//
// Flow per tick:
//  1. SELECT up to relayBatchSize rows WHERE status='pending' ORDER BY created_at
//  2. For each row: call publisher.Publish()
//     - success → UPDATE status='published'
//     - failure → UPDATE status='failed', log error, move to next row
//
// The relay is broker-agnostic — it depends only on IEventPublisher.
// Which broker receives the events is decided at startup in container/container.go
// (currently kafkaProducer; could be swapped for rabbitmq publisher with no changes here).
type OutboxRelay struct {
	repo      domainRepo.IOutboxRepository
	publisher domainService.IEventPublisher
}

// NewOutboxRelay wires the outbox repository and event publisher together.
// Called once at startup from the container.
func NewOutboxRelay(repo domainRepo.IOutboxRepository, publisher domainService.IEventPublisher) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher}
}

// Start blocks, running process() on every relayInterval tick until ctx is cancelled.
// Runs in its own goroutine inside the worker errgroup.
func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(relayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown — context cancelled by SIGINT/SIGTERM.
			return
		case <-ticker.C:
			r.process(ctx)
		}
	}
}

// ProcessOnce exposes a single relay tick for integration tests.
func (r *OutboxRelay) ProcessOnce(ctx context.Context) { r.process(ctx) }

// process fetches one batch of pending outbox events and publishes each one.
// Errors on individual events are logged and marked failed; the relay continues
// to the next event rather than aborting the whole batch.
func (r *OutboxRelay) process(ctx context.Context) {
	events, err := r.repo.ListPending(ctx, relayBatchSize)
	if err != nil {
		slog.ErrorContext(ctx, "outbox relay: list pending failed", "error", err)
		return
	}

	for _, event := range events {
		if err := r.publisher.Publish(ctx, event); err != nil {
			slog.ErrorContext(ctx, "outbox relay: publish failed", "event_id", event.EventID, "error", err)
			// Mark as failed so operators can observe and retry; does not block other events.
			if markErr := r.repo.MarkFailed(ctx, event.ID); markErr != nil {
				slog.ErrorContext(ctx, "outbox relay: mark failed error", "event_id", event.EventID, "error", markErr)
			}
			continue
		}

		// Only mark published after the broker confirmed receipt.
		// If this update fails the row stays pending and will be published again
		// on the next tick (at-least-once delivery — consumers must be idempotent).
		if err := r.repo.MarkPublished(ctx, event.ID); err != nil {
			slog.ErrorContext(ctx, "outbox relay: mark published failed", "event_id", event.EventID, "error", err)
		}
	}
}
