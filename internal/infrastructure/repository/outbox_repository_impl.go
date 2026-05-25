package repository

import (
	"context"
	"fmt"
	"strings"

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

func (r *outboxRepository) CreateEventWithDeliveries(ctx context.Context, event *domainModel.OutboxEvent, destinations []string) error {
	conn := r.conn(ctx)

	res, err := conn.ExecContext(ctx,
		`INSERT INTO outbox_events (event_id, event_type, aggregate_type, aggregate_id, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, NOW(3))`,
		event.EventID, event.EventType, event.AggregateType, event.AggregateID, event.Payload,
	)
	if err != nil {
		return err
	}

	eventID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	if len(destinations) == 0 {
		return nil
	}

	placeholders := make([]string, len(destinations))
	args := make([]any, 0, len(destinations)*2)
	for i, dest := range destinations {
		placeholders[i] = "(?, ?)"
		args = append(args, eventID, dest)
	}

	_, err = conn.ExecContext(ctx,
		fmt.Sprintf(
			`INSERT INTO outbox_deliveries (outbox_event_id, destination) VALUES %s`,
			strings.Join(placeholders, ", "),
		),
		args...,
	)
	return err
}

func (r *outboxRepository) ListPendingDeliveries(ctx context.Context, destination string, limit int) ([]*domainModel.OutboxDelivery, error) {
	var deliveries []*domainModel.OutboxDelivery
	err := r.conn(ctx).SelectContext(ctx, &deliveries,
		`SELECT id, outbox_event_id, destination, status, attempt_count, last_error, published_at, created_at, updated_at
		 FROM outbox_deliveries
		 WHERE destination = ? AND status = ?
		 ORDER BY id ASC
		 LIMIT ?`,
		destination, domainModel.OutboxDeliveryStatusPending, limit,
	)
	return deliveries, err
}

func (r *outboxRepository) GetEventByID(ctx context.Context, id uint) (*domainModel.OutboxEvent, error) {
	var event domainModel.OutboxEvent
	err := r.conn(ctx).GetContext(ctx, &event,
		`SELECT id, event_id, event_type, aggregate_type, aggregate_id, payload, created_at
		 FROM outbox_events WHERE id = ?`,
		id,
	)
	return &event, err
}

func (r *outboxRepository) MarkDeliveryPublished(ctx context.Context, deliveryID uint) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE outbox_deliveries
		 SET status = ?, published_at = NOW(3), attempt_count = attempt_count + 1
		 WHERE id = ?`,
		domainModel.OutboxDeliveryStatusPublished, deliveryID,
	)
	return err
}

func (r *outboxRepository) MarkDeliveryFailed(ctx context.Context, deliveryID uint, errMsg string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE outbox_deliveries
		 SET status = ?, last_error = ?, attempt_count = attempt_count + 1
		 WHERE id = ?`,
		domainModel.OutboxDeliveryStatusFailed, errMsg, deliveryID,
	)
	return err
}
