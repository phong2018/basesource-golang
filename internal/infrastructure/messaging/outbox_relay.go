package messaging

import (
	"context"
	"log/slog"
	"time"

	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

const (
	relayBatchSize = 10
	relayInterval  = 2 * time.Second
)

type OutboxRelay struct {
	repo      domainRepo.IOutboxRepository
	publisher domainService.IEventPublisher
}

func NewOutboxRelay(repo domainRepo.IOutboxRepository, publisher domainService.IEventPublisher) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(relayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.process(ctx)
		}
	}
}

func (r *OutboxRelay) process(ctx context.Context) {
	events, err := r.repo.ListPending(ctx, relayBatchSize)
	if err != nil {
		slog.ErrorContext(ctx, "outbox relay: list pending failed", "error", err)
		return
	}

	for _, event := range events {
		if err := r.publisher.Publish(ctx, event); err != nil {
			slog.ErrorContext(ctx, "outbox relay: publish failed", "event_id", event.EventID, "error", err)
			if markErr := r.repo.MarkFailed(ctx, event.ID); markErr != nil {
				slog.ErrorContext(ctx, "outbox relay: mark failed error", "event_id", event.EventID, "error", markErr)
			}
			continue
		}

		if err := r.repo.MarkPublished(ctx, event.ID); err != nil {
			slog.ErrorContext(ctx, "outbox relay: mark published failed", "event_id", event.EventID, "error", err)
		}
	}
}
