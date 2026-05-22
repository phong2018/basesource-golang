package service

import (
	"context"
	"github.com/yourname/go-clean-base/internal/domain/model"
)

type INotificationClient interface {
	Send(ctx context.Context, n *model.Notification) (string, error)
}
