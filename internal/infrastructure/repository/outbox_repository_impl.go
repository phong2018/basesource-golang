package repository

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

type outboxRepository struct {
	baseRepository
}

func NewOutboxRepository(db *database.Client) domainRepo.IOutboxRepository {
	return &outboxRepository{baseRepository{db: db}}
}

func (r *outboxRepository) Create(ctx context.Context, event *domainModel.OutboxEvent) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO outbox_events (event_id, event_type, aggregate_id, payload, status, created_at)
		 VALUES (?, ?, ?, ?, 'pending', NOW())`,
		event.EventID, event.EventType, event.AggregateID, event.Payload,
	)
	return err
}

func (r *outboxRepository) ListPending(ctx context.Context, limit int) ([]*domainModel.OutboxEvent, error) {
	var events []*domainModel.OutboxEvent
	err := r.conn(ctx).SelectContext(ctx, &events,
		`SELECT id, event_id, event_type, aggregate_id, payload, status, created_at, published_at
		 FROM outbox_events
		 WHERE status = 'pending'
		 ORDER BY created_at ASC
		 LIMIT ?`,
		limit,
	)
	return events, err
}

func (r *outboxRepository) MarkPublished(ctx context.Context, id uint) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE outbox_events SET status = 'published', published_at = NOW() WHERE id = ?`,
		id,
	)
	return err
}

func (r *outboxRepository) MarkFailed(ctx context.Context, id uint) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE outbox_events SET status = 'failed' WHERE id = ?`,
		id,
	)
	return err
}
