package service

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type IEventPublisher interface {
	Publish(ctx context.Context, event *model.OutboxEvent) error
	Close() error
}
