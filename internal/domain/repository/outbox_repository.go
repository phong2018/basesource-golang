package repository

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type IOutboxRepository interface {
	Create(ctx context.Context, event *model.OutboxEvent) error
	ListPending(ctx context.Context, limit int) ([]*model.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uint) error
	MarkFailed(ctx context.Context, id uint) error
}
