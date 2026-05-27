package usecase

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
	"github.com/yourname/go-clean-base/pkg/helper"
)

type todoOwnedUsecase struct {
	repo        domainRepo.ITodoOwnedRepository
	userRepo    domainRepo.IUserRepository
	commentRepo domainRepo.ITodoCommentRepository
}

func NewTodoOwnedUsecase(
	repo domainRepo.ITodoOwnedRepository,
	userRepo domainRepo.IUserRepository,
	commentRepo domainRepo.ITodoCommentRepository,
) ITodoOwnedUsecase {
	return &todoOwnedUsecase{repo: repo, userRepo: userRepo, commentRepo: commentRepo}
}

func (u *todoOwnedUsecase) ListMine(ctx context.Context, ownerID int64, filter dto.ListTodoInput) ([]*dto.OwnedTodoOutput, error) {
	f := domainModel.TodoFilter{Done: filter.Done, Search: filter.Search, SortBy: filter.SortBy}
	todos, err := u.repo.ListByOwner(ctx, ownerID, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*dto.OwnedTodoOutput, len(todos))
	for i, t := range todos {
		out[i] = mapOwnedTodo(t)
	}
	return out, nil
}

func (u *todoOwnedUsecase) GetMine(ctx context.Context, id uint, ownerID int64) (*dto.OwnedTodoOutput, error) {
	t, err := u.repo.FindOwned(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	return mapOwnedTodo(t), nil
}

func (u *todoOwnedUsecase) CreateMine(ctx context.Context, input dto.CreateOwnedTodoInput) (*dto.OwnedTodoOutput, error) {
	t := &domainModel.OwnedTodo{
		OwnerID:     &input.OwnerID,
		Title:       input.Title,
		Description: input.Description,
	}
	if err := u.repo.CreateOwned(ctx, t); err != nil {
		return nil, apperror.Internal(err)
	}
	created, err := u.repo.FindOwned(ctx, t.ID, input.OwnerID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return mapOwnedTodo(created), nil
}

func (u *todoOwnedUsecase) UpdateMine(ctx context.Context, input dto.UpdateOwnedTodoInput) (*dto.OwnedTodoOutput, error) {
	t, err := u.repo.FindOwned(ctx, input.ID, input.OwnerID)
	if err != nil {
		return nil, err
	}
	t.Title = input.Title
	t.Done = input.Done
	if err := u.repo.UpdateOwned(ctx, t); err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := u.repo.FindOwned(ctx, input.ID, input.OwnerID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return mapOwnedTodo(updated), nil
}

func (u *todoOwnedUsecase) DeleteMine(ctx context.Context, id uint, ownerID int64) error {
	return u.repo.SoftDeleteOwned(ctx, id, ownerID)
}

func (u *todoOwnedUsecase) DeleteMineWhere(ctx context.Context, ownerID int64, condition string) (int64, error) {
	return u.repo.SoftDeleteWhere(ctx, ownerID, condition)
}

func (u *todoOwnedUsecase) UpdateMineField(ctx context.Context, id uint, ownerID int64, field, value string) error {
	return u.repo.UpdateField(ctx, id, ownerID, field, value)
}

func (u *todoOwnedUsecase) CountByTitleFilter(ctx context.Context, titleFilter string) (int, error) {
	count, err := u.repo.CountByTitleFilter(ctx, titleFilter)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return count, nil
}

func (u *todoOwnedUsecase) BulkDelete(ctx context.Context, input dto.BulkDeleteInput) error {
	return u.repo.BulkSoftDelete(ctx, input.IDs)
}

func (u *todoOwnedUsecase) BulkSetStatus(ctx context.Context, input dto.BulkStatusInput) error {
	return u.repo.BulkSetStatus(ctx, input.IDs, input.Done, input.OrderBy)
}

func (u *todoOwnedUsecase) ShareTodo(ctx context.Context, input dto.ShareTodoInput) error {
	if _, err := u.repo.FindOwned(ctx, input.TodoID, input.OwnerID); err != nil {
		return err
	}
	target, err := u.userRepo.FindByEmail(ctx, input.TargetEmail)
	if err != nil {
		return apperror.NotFound("target user not found")
	}
	return u.repo.Share(ctx, input.TodoID, target.ID)
}

func (u *todoOwnedUsecase) RevokeShare(ctx context.Context, todoID uint, ownerID int64, targetUserID int64) error {
	if _, err := u.repo.FindOwned(ctx, todoID, ownerID); err != nil {
		return err
	}
	return u.repo.RevokeShare(ctx, todoID, targetUserID)
}

func (u *todoOwnedUsecase) ListComments(ctx context.Context, todoID uint) ([]*dto.CommentOutput, error) {
	comments, err := u.commentRepo.List(ctx, todoID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*dto.CommentOutput, len(comments))
	for i, c := range comments {
		out[i] = mapComment(c)
	}
	return out, nil
}

func (u *todoOwnedUsecase) ListCommentsSorted(ctx context.Context, todoID uint, orderBy string) ([]*dto.CommentOutput, error) {
	comments, err := u.commentRepo.ListSorted(ctx, todoID, orderBy)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.CommentOutput, len(comments))
	for i, c := range comments {
		out[i] = mapComment(c)
	}
	return out, nil
}

func (u *todoOwnedUsecase) AddComment(ctx context.Context, input dto.AddCommentInput) (*dto.CommentOutput, error) {
	c := &domainModel.TodoComment{
		TodoID: input.TodoID,
		UserID: input.CallerID,
		Body:   input.Body,
	}
	if err := u.commentRepo.Create(ctx, c); err != nil {
		return nil, apperror.Internal(err)
	}
	created, err := u.commentRepo.FindByID(ctx, c.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return mapComment(created), nil
}

func (u *todoOwnedUsecase) DeleteComment(ctx context.Context, commentID uint, callerID int64, isAdmin bool) error {
	c, err := u.commentRepo.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	if c.UserID != callerID && !isAdmin {
		return apperror.Forbidden("cannot delete another user's comment")
	}
	return u.commentRepo.Delete(ctx, commentID)
}

func mapOwnedTodo(t *domainModel.OwnedTodo) *dto.OwnedTodoOutput {
	return &dto.OwnedTodoOutput{
		ID:            t.ID,
		Title:         t.Title,
		Description:   t.Description,
		Done:          t.Done,
		AttachmentURL: t.AttachmentURL,
		CreatedAt:     helper.FormatTime(t.CreatedAt),
		UpdatedAt:     helper.FormatTime(t.UpdatedAt),
	}
}

func mapComment(c *domainModel.TodoComment) *dto.CommentOutput {
	return &dto.CommentOutput{
		ID:        c.ID,
		TodoID:    c.TodoID,
		UserID:    c.UserID,
		Body:      c.Body,
		CreatedAt: helper.FormatTime(c.CreatedAt),
	}
}
