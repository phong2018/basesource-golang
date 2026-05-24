package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

type NotificationPublisherMock struct {
	PublishNotificationFn func(ctx context.Context, n *domainModel.Notification) error
}

var _ domainService.INotificationPublisher = (*NotificationPublisherMock)(nil)

func (m *NotificationPublisherMock) PublishNotification(ctx context.Context, n *domainModel.Notification) error {
	if m.PublishNotificationFn != nil {
		return m.PublishNotificationFn(ctx, n)
	}
	return nil
}

func (m *NotificationPublisherMock) Close() error { return nil }
