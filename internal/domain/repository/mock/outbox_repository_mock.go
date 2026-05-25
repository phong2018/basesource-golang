package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type OutboxRepositoryMock struct {
	CreateEventWithDeliveriesFn func(ctx context.Context, event *domainModel.OutboxEvent, destinations []string) error
	ListPendingDeliveriesFn     func(ctx context.Context, destination string, limit int) ([]*domainModel.OutboxDelivery, error)
	GetEventByIDFn              func(ctx context.Context, id uint) (*domainModel.OutboxEvent, error)
	MarkDeliveryPublishedFn     func(ctx context.Context, deliveryID uint) error
	MarkDeliveryFailedFn        func(ctx context.Context, deliveryID uint, errMsg string) error
}

var _ domainRepo.IOutboxRepository = (*OutboxRepositoryMock)(nil)

func (m *OutboxRepositoryMock) CreateEventWithDeliveries(ctx context.Context, event *domainModel.OutboxEvent, destinations []string) error {
	if m.CreateEventWithDeliveriesFn != nil {
		return m.CreateEventWithDeliveriesFn(ctx, event, destinations)
	}
	return nil
}

func (m *OutboxRepositoryMock) ListPendingDeliveries(ctx context.Context, destination string, limit int) ([]*domainModel.OutboxDelivery, error) {
	if m.ListPendingDeliveriesFn != nil {
		return m.ListPendingDeliveriesFn(ctx, destination, limit)
	}
	return nil, nil
}

func (m *OutboxRepositoryMock) GetEventByID(ctx context.Context, id uint) (*domainModel.OutboxEvent, error) {
	if m.GetEventByIDFn != nil {
		return m.GetEventByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *OutboxRepositoryMock) MarkDeliveryPublished(ctx context.Context, deliveryID uint) error {
	if m.MarkDeliveryPublishedFn != nil {
		return m.MarkDeliveryPublishedFn(ctx, deliveryID)
	}
	return nil
}

func (m *OutboxRepositoryMock) MarkDeliveryFailed(ctx context.Context, deliveryID uint, errMsg string) error {
	if m.MarkDeliveryFailedFn != nil {
		return m.MarkDeliveryFailedFn(ctx, deliveryID, errMsg)
	}
	return nil
}
