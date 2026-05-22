package repository

import (
	"context"
	"github.com/yourname/go-clean-base/internal/domain/model"
)

type ITodoRepository interface {
	GetByID(ctx context.Context, id uint) (*model.Todo, error)
	List(ctx context.Context, filter model.TodoFilter, page model.Pagination) ([]*model.Todo, error)
	Create(ctx context.Context, todo *model.Todo) (*model.Todo, error)
	Update(ctx context.Context, todo *model.Todo) (*model.Todo, error)
	Delete(ctx context.Context, id uint) error
}
