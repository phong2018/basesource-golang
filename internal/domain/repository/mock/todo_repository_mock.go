package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type TodoRepositoryMock struct {
	GetByIDFn func(ctx context.Context, id uint) (*domainModel.Todo, error)
	ListFn    func(ctx context.Context, filter domainModel.TodoFilter, page domainModel.Pagination) ([]*domainModel.Todo, error)
	CreateFn  func(ctx context.Context, todo *domainModel.Todo) (*domainModel.Todo, error)
	UpdateFn  func(ctx context.Context, todo *domainModel.Todo) (*domainModel.Todo, error)
	DeleteFn  func(ctx context.Context, id uint) error
}

var _ domainRepo.ITodoRepository = (*TodoRepositoryMock)(nil)

func (m *TodoRepositoryMock) GetByID(ctx context.Context, id uint) (*domainModel.Todo, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *TodoRepositoryMock) List(ctx context.Context, f domainModel.TodoFilter, p domainModel.Pagination) ([]*domainModel.Todo, error) {
	return m.ListFn(ctx, f, p)
}
func (m *TodoRepositoryMock) Create(ctx context.Context, todo *domainModel.Todo) (*domainModel.Todo, error) {
	return m.CreateFn(ctx, todo)
}
func (m *TodoRepositoryMock) Update(ctx context.Context, todo *domainModel.Todo) (*domainModel.Todo, error) {
	return m.UpdateFn(ctx, todo)
}
func (m *TodoRepositoryMock) Delete(ctx context.Context, id uint) error {
	return m.DeleteFn(ctx, id)
}
