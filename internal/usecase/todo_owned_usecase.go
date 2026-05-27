package usecase

import (
	"context"

	"github.com/yourname/go-clean-base/internal/usecase/dto"
)

type ITodoOwnedUsecase interface {
	ListMine(ctx context.Context, ownerID int64, filter dto.ListTodoInput) ([]*dto.OwnedTodoOutput, error)
	GetMine(ctx context.Context, id uint, ownerID int64) (*dto.OwnedTodoOutput, error)
	CreateMine(ctx context.Context, input dto.CreateOwnedTodoInput) (*dto.OwnedTodoOutput, error)
	UpdateMine(ctx context.Context, input dto.UpdateOwnedTodoInput) (*dto.OwnedTodoOutput, error)
	DeleteMine(ctx context.Context, id uint, ownerID int64) error

	BulkDelete(ctx context.Context, input dto.BulkDeleteInput) error
	BulkSetStatus(ctx context.Context, input dto.BulkStatusInput) error

	ShareTodo(ctx context.Context, input dto.ShareTodoInput) error
	RevokeShare(ctx context.Context, todoID uint, ownerID int64, targetUserID int64) error

	ListComments(ctx context.Context, todoID uint) ([]*dto.CommentOutput, error)
	AddComment(ctx context.Context, input dto.AddCommentInput) (*dto.CommentOutput, error)
	DeleteComment(ctx context.Context, commentID uint, callerID int64, isAdmin bool) error
}
