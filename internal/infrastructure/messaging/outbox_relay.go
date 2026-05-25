package messaging

import (
	"context"
	"log/slog"
	"time"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

const (
	relayBatchSize = 10
	relayInterval  = 2 * time.Second
)

// OutboxRelay polls outbox_deliveries for a specific destination broker and
// publishes each pending event via the injected publisher.
// Use NewKafkaOutboxRelay or NewRabbitMQOutboxRelay to construct — never set
// destination directly.
type OutboxRelay struct {
	repo        domainRepo.IOutboxRepository
	publisher   domainService.IEventPublisher
	destination string
}

// NewKafkaOutboxRelay returns a relay that processes kafka-destined deliveries.
func NewKafkaOutboxRelay(repo domainRepo.IOutboxRepository, publisher domainService.IEventPublisher) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, destination: domainModel.OutboxDestinationKafka}
}

// NewRabbitMQOutboxRelay returns a relay that processes rabbitmq-destined deliveries.
func NewRabbitMQOutboxRelay(repo domainRepo.IOutboxRepository, publisher domainService.IEventPublisher) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, destination: domainModel.OutboxDestinationRabbitMQ}
}

// Start blocks, running process() on every relayInterval tick until ctx is cancelled.
func (r *OutboxRelay) Start(ctx context.Context) {
	slog.InfoContext(ctx, "outbox relay starting", "destination", r.destination)
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

// ProcessOnce exposes a single relay tick for integration tests.
func (r *OutboxRelay) ProcessOnce(ctx context.Context) { r.process(ctx) }

func (r *OutboxRelay) process(ctx context.Context) {
	deliveries, err := r.repo.ListPendingDeliveries(ctx, r.destination, relayBatchSize)
	if err != nil {
		slog.ErrorContext(ctx, "outbox relay: list pending deliveries failed", "destination", r.destination, "error", err)
		return
	}

	for _, d := range deliveries {
		event, err := r.repo.GetEventByID(ctx, d.OutboxEventID)
		if err != nil {
			slog.ErrorContext(ctx, "outbox relay: get event failed", "destination", r.destination, "delivery_id", d.ID, "error", err)
			if markErr := r.repo.MarkDeliveryFailed(ctx, d.ID, err.Error()); markErr != nil {
				slog.ErrorContext(ctx, "outbox relay: mark delivery failed error", "destination", r.destination, "delivery_id", d.ID, "error", markErr)
			}
			continue
		}

		if err := r.publisher.Publish(ctx, event); err != nil {
			// Leave delivery as pending — transient broker failures are retried on the next tick.
			slog.ErrorContext(ctx, "outbox relay: publish failed, will retry", "destination", r.destination, "delivery_id", d.ID, "event_id", event.EventID, "error", err)
			continue
		}

		if markErr := r.repo.MarkDeliveryPublished(ctx, d.ID); markErr != nil {
			slog.ErrorContext(ctx, "outbox relay: mark delivery published error", "destination", r.destination, "delivery_id", d.ID, "error", markErr)
		}
	}
}
