package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type TodoCommentRepositoryMock struct {
	ListFn       func(ctx context.Context, todoID uint) ([]*domainModel.TodoComment, error)
	ListSortedFn func(ctx context.Context, todoID uint, orderBy string) ([]*domainModel.TodoComment, error)
	CreateFn     func(ctx context.Context, comment *domainModel.TodoComment) error
	FindByIDFn   func(ctx context.Context, id uint) (*domainModel.TodoComment, error)
	DeleteFn     func(ctx context.Context, id uint) error
}

var _ domainRepo.ITodoCommentRepository = (*TodoCommentRepositoryMock)(nil)

func (m *TodoCommentRepositoryMock) List(ctx context.Context, todoID uint) ([]*domainModel.TodoComment, error) {
	return m.ListFn(ctx, todoID)
}
func (m *TodoCommentRepositoryMock) ListSorted(ctx context.Context, todoID uint, orderBy string) ([]*domainModel.TodoComment, error) {
	return m.ListSortedFn(ctx, todoID, orderBy)
}
func (m *TodoCommentRepositoryMock) Create(ctx context.Context, comment *domainModel.TodoComment) error {
	return m.CreateFn(ctx, comment)
}
func (m *TodoCommentRepositoryMock) FindByID(ctx context.Context, id uint) (*domainModel.TodoComment, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *TodoCommentRepositoryMock) Delete(ctx context.Context, id uint) error {
	return m.DeleteFn(ctx, id)
}
