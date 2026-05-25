package repository

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type IOutboxRepository interface {
	// write path — called inside a usecase transaction alongside the business write
	CreateEventWithDeliveries(ctx context.Context, event *model.OutboxEvent, destinations []string) error

	// relay read path — scoped per destination broker
	ListPendingDeliveries(ctx context.Context, destination string, limit int) ([]*model.OutboxDelivery, error)
	GetEventByID(ctx context.Context, id uint) (*model.OutboxEvent, error)

	// relay update path
	MarkDeliveryPublished(ctx context.Context, deliveryID uint) error
	MarkDeliveryFailed(ctx context.Context, deliveryID uint, errMsg string) error
}
