package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type TodoOwnedRepositoryMock struct {
	ListByOwnerFn      func(ctx context.Context, ownerID int64, filter domainModel.TodoFilter) ([]*domainModel.OwnedTodo, error)
	FindOwnedFn        func(ctx context.Context, id uint, ownerID int64) (*domainModel.OwnedTodo, error)
	CreateOwnedFn      func(ctx context.Context, todo *domainModel.OwnedTodo) error
	UpdateOwnedFn      func(ctx context.Context, todo *domainModel.OwnedTodo) error
	SoftDeleteOwnedFn  func(ctx context.Context, id uint, ownerID int64) error
	BulkSoftDeleteFn   func(ctx context.Context, ids []uint) error
	BulkSetStatusFn    func(ctx context.Context, ids []uint, done bool) error
	ShareFn            func(ctx context.Context, todoID uint, targetUserID int64) error
	RevokeShareFn      func(ctx context.Context, todoID uint, targetUserID int64) error
	UpdateAttachmentFn func(ctx context.Context, id uint, ownerID int64, url *string) error
}

var _ domainRepo.ITodoOwnedRepository = (*TodoOwnedRepositoryMock)(nil)

func (m *TodoOwnedRepositoryMock) ListByOwner(ctx context.Context, ownerID int64, filter domainModel.TodoFilter) ([]*domainModel.OwnedTodo, error) {
	return m.ListByOwnerFn(ctx, ownerID, filter)
}
func (m *TodoOwnedRepositoryMock) FindOwned(ctx context.Context, id uint, ownerID int64) (*domainModel.OwnedTodo, error) {
	return m.FindOwnedFn(ctx, id, ownerID)
}
func (m *TodoOwnedRepositoryMock) CreateOwned(ctx context.Context, todo *domainModel.OwnedTodo) error {
	return m.CreateOwnedFn(ctx, todo)
}
func (m *TodoOwnedRepositoryMock) UpdateOwned(ctx context.Context, todo *domainModel.OwnedTodo) error {
	return m.UpdateOwnedFn(ctx, todo)
}
func (m *TodoOwnedRepositoryMock) SoftDeleteOwned(ctx context.Context, id uint, ownerID int64) error {
	return m.SoftDeleteOwnedFn(ctx, id, ownerID)
}
func (m *TodoOwnedRepositoryMock) BulkSoftDelete(ctx context.Context, ids []uint) error {
	return m.BulkSoftDeleteFn(ctx, ids)
}
func (m *TodoOwnedRepositoryMock) BulkSetStatus(ctx context.Context, ids []uint, done bool) error {
	return m.BulkSetStatusFn(ctx, ids, done)
}
func (m *TodoOwnedRepositoryMock) Share(ctx context.Context, todoID uint, targetUserID int64) error {
	return m.ShareFn(ctx, todoID, targetUserID)
}
func (m *TodoOwnedRepositoryMock) RevokeShare(ctx context.Context, todoID uint, targetUserID int64) error {
	return m.RevokeShareFn(ctx, todoID, targetUserID)
}
func (m *TodoOwnedRepositoryMock) UpdateAttachment(ctx context.Context, id uint, ownerID int64, url *string) error {
	return m.UpdateAttachmentFn(ctx, id, ownerID, url)
}
