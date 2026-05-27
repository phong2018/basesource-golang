package repository

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type ITodoCommentRepository interface {
	List(ctx context.Context, todoID uint) ([]*model.TodoComment, error)
	Create(ctx context.Context, comment *model.TodoComment) error
	FindByID(ctx context.Context, id uint) (*model.TodoComment, error)
	Delete(ctx context.Context, id uint) error
}
