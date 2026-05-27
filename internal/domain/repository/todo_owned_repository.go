package repository

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type ITodoOwnedRepository interface {
	ListByOwner(ctx context.Context, ownerID int64, filter model.TodoFilter) ([]*model.OwnedTodo, error)
	FindOwned(ctx context.Context, id uint, ownerID int64) (*model.OwnedTodo, error)
	CreateOwned(ctx context.Context, todo *model.OwnedTodo) error
	UpdateOwned(ctx context.Context, todo *model.OwnedTodo) error
	SoftDeleteOwned(ctx context.Context, id uint, ownerID int64) error
	BulkSoftDelete(ctx context.Context, ids []uint) error
	BulkSetStatus(ctx context.Context, ids []uint, done bool) error
	Share(ctx context.Context, todoID uint, targetUserID int64) error
	RevokeShare(ctx context.Context, todoID uint, targetUserID int64) error
	UpdateAttachment(ctx context.Context, id uint, ownerID int64, url *string) error
}
