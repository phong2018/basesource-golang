package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type OutboxRepositoryMock struct {
	CreateFn        func(ctx context.Context, event *domainModel.OutboxEvent) error
	ListPendingFn   func(ctx context.Context, limit int) ([]*domainModel.OutboxEvent, error)
	MarkPublishedFn func(ctx context.Context, id uint) error
	MarkFailedFn    func(ctx context.Context, id uint) error
}

var _ domainRepo.IOutboxRepository = (*OutboxRepositoryMock)(nil)

func (m *OutboxRepositoryMock) Create(ctx context.Context, event *domainModel.OutboxEvent) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, event)
	}
	return nil
}
func (m *OutboxRepositoryMock) ListPending(ctx context.Context, limit int) ([]*domainModel.OutboxEvent, error) {
	if m.ListPendingFn != nil {
		return m.ListPendingFn(ctx, limit)
	}
	return nil, nil
}
func (m *OutboxRepositoryMock) MarkPublished(ctx context.Context, id uint) error {
	if m.MarkPublishedFn != nil {
		return m.MarkPublishedFn(ctx, id)
	}
	return nil
}
func (m *OutboxRepositoryMock) MarkFailed(ctx context.Context, id uint) error {
	if m.MarkFailedFn != nil {
		return m.MarkFailedFn(ctx, id)
	}
	return nil
}
