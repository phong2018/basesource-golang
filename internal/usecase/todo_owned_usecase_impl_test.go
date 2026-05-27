package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	repoMock "github.com/yourname/go-clean-base/internal/domain/repository/mock"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

func fixedOwned(id uint, ownerID int64) *domainModel.OwnedTodo {
	return &domainModel.OwnedTodo{
		ID:        id,
		OwnerID:   &ownerID,
		Title:     "Test todo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func fixedComment(id, todoID uint, userID int64) *domainModel.TodoComment {
	return &domainModel.TodoComment{
		ID:        id,
		TodoID:    todoID,
		UserID:    userID,
		Body:      "a comment",
		CreatedAt: time.Now(),
	}
}

func newOwnedUsecase(
	repo *repoMock.TodoOwnedRepositoryMock,
	userRepo *repoMock.UserRepositoryMock,
	commentRepo *repoMock.TodoCommentRepositoryMock,
) usecase.ITodoOwnedUsecase {
	return usecase.NewTodoOwnedUsecase(repo, userRepo, commentRepo)
}

// ── GetMine ───────────────────────────────────────────────────────────────────

func TestTodoOwned_GetMine_ok(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, id uint, ownerID int64) (*domainModel.OwnedTodo, error) {
				return fixedOwned(id, ownerID), nil
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	out, err := uc.GetMine(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.ID != 1 {
		t.Errorf("expected id=1, got %d", out.ID)
	}
}

func TestTodoOwned_GetMine_wrongOwner(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, _ uint, _ int64) (*domainModel.OwnedTodo, error) {
				return nil, apperror.Forbidden("not owner")
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	_, err := uc.GetMine(context.Background(), 1, 99)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 403 {
		t.Errorf("expected 403 Forbidden, got %v", err)
	}
}

func TestTodoOwned_GetMine_notFound(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, _ uint, _ int64) (*domainModel.OwnedTodo, error) {
				return nil, apperror.NotFound("todo not found")
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	_, err := uc.GetMine(context.Background(), 999, 1)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404 NotFound, got %v", err)
	}
}

// ── CreateMine ────────────────────────────────────────────────────────────────

func TestTodoOwned_CreateMine_ok(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			CreateOwnedFn: func(_ context.Context, t *domainModel.OwnedTodo) error {
				t.ID = 5
				return nil
			},
			FindOwnedFn: func(_ context.Context, id uint, ownerID int64) (*domainModel.OwnedTodo, error) {
				return fixedOwned(id, ownerID), nil
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	out, err := uc.CreateMine(context.Background(), dto.CreateOwnedTodoInput{OwnerID: 1, Title: "New"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out == nil {
		t.Error("expected output, got nil")
	}
}

// ── UpdateMine ────────────────────────────────────────────────────────────────

func TestTodoOwned_UpdateMine_wrongOwner(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, _ uint, _ int64) (*domainModel.OwnedTodo, error) {
				return nil, apperror.Forbidden("not owner")
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	_, err := uc.UpdateMine(context.Background(), dto.UpdateOwnedTodoInput{ID: 1, OwnerID: 99})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 403 {
		t.Errorf("expected 403 Forbidden, got %v", err)
	}
}

// ── DeleteMine ────────────────────────────────────────────────────────────────

func TestTodoOwned_DeleteMine_softDeletes(t *testing.T) {
	called := false
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			SoftDeleteOwnedFn: func(_ context.Context, id uint, ownerID int64) error {
				called = true
				return nil
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	if err := uc.DeleteMine(context.Background(), 1, 42); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected SoftDeleteOwned to be called")
	}
}

func TestTodoOwned_DeleteMine_wrongOwner(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			SoftDeleteOwnedFn: func(_ context.Context, _ uint, _ int64) error {
				return apperror.Forbidden("not owner")
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	err := uc.DeleteMine(context.Background(), 1, 99)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 403 {
		t.Errorf("expected 403 Forbidden, got %v", err)
	}
}

// ── BulkDelete ────────────────────────────────────────────────────────────────

func TestTodoOwned_BulkDelete_ok(t *testing.T) {
	called := false
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			BulkSoftDeleteFn: func(_ context.Context, ids []uint) error {
				called = true
				return nil
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	if err := uc.BulkDelete(context.Background(), dto.BulkDeleteInput{IDs: []uint{1, 2}}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected BulkSoftDelete to be called")
	}
}

// ── BulkSetStatus ─────────────────────────────────────────────────────────────

func TestTodoOwned_BulkSetStatus_ok(t *testing.T) {
	called := false
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			BulkSetStatusFn: func(_ context.Context, ids []uint, done bool, orderBy string) error {
				called = true
				if !done {
					return errors.New("expected done=true")
				}
				return nil
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	if err := uc.BulkSetStatus(context.Background(), dto.BulkStatusInput{IDs: []uint{1}, Done: true}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected BulkSetStatus to be called")
	}
}

// ── ShareTodo ─────────────────────────────────────────────────────────────────

func TestTodoOwned_ShareTodo_ok(t *testing.T) {
	shareCalled := false
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, id uint, ownerID int64) (*domainModel.OwnedTodo, error) {
				return fixedOwned(id, ownerID), nil
			},
			ShareFn: func(_ context.Context, _ uint, _ int64) error {
				shareCalled = true
				return nil
			},
		},
		&repoMock.UserRepositoryMock{
			FindByEmailFn: func(_ context.Context, _ string) (*domainModel.User, error) {
				return fixedUser(2, domainModel.RoleUser), nil
			},
		},
		&repoMock.TodoCommentRepositoryMock{},
	)
	if err := uc.ShareTodo(context.Background(), dto.ShareTodoInput{TodoID: 1, OwnerID: 1, TargetEmail: "b@b.com"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !shareCalled {
		t.Error("expected Share to be called")
	}
}

func TestTodoOwned_ShareTodo_unknownEmail(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, id uint, ownerID int64) (*domainModel.OwnedTodo, error) {
				return fixedOwned(id, ownerID), nil
			},
		},
		&repoMock.UserRepositoryMock{
			FindByEmailFn: func(_ context.Context, _ string) (*domainModel.User, error) {
				return nil, errors.New("not found")
			},
		},
		&repoMock.TodoCommentRepositoryMock{},
	)
	err := uc.ShareTodo(context.Background(), dto.ShareTodoInput{TodoID: 1, OwnerID: 1, TargetEmail: "ghost@ghost.com"})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404 NotFound, got %v", err)
	}
}

func TestTodoOwned_ShareTodo_notOwner(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{
			FindOwnedFn: func(_ context.Context, _ uint, _ int64) (*domainModel.OwnedTodo, error) {
				return nil, apperror.Forbidden("not owner")
			},
		},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{},
	)
	err := uc.ShareTodo(context.Background(), dto.ShareTodoInput{TodoID: 1, OwnerID: 99, TargetEmail: "b@b.com"})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 403 {
		t.Errorf("expected 403 Forbidden, got %v", err)
	}
}

// ── AddComment ────────────────────────────────────────────────────────────────

func TestTodoOwned_AddComment_ok(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{
			CreateFn: func(_ context.Context, c *domainModel.TodoComment) error {
				c.ID = 10
				return nil
			},
			FindByIDFn: func(_ context.Context, id uint) (*domainModel.TodoComment, error) {
				return fixedComment(id, 1, 5), nil
			},
		},
	)
	out, err := uc.AddComment(context.Background(), dto.AddCommentInput{TodoID: 1, CallerID: 5, Body: "hello"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.Body != "a comment" {
		t.Errorf("unexpected body: %s", out.Body)
	}
}

// ── DeleteComment ─────────────────────────────────────────────────────────────

func TestTodoOwned_DeleteComment_ownComment(t *testing.T) {
	deleteCalled := false
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{
			FindByIDFn: func(_ context.Context, id uint) (*domainModel.TodoComment, error) {
				return fixedComment(id, 1, 5), nil
			},
			DeleteFn: func(_ context.Context, _ uint) error {
				deleteCalled = true
				return nil
			},
		},
	)
	if err := uc.DeleteComment(context.Background(), 1, 5, false); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !deleteCalled {
		t.Error("expected Delete to be called")
	}
}

func TestTodoOwned_DeleteComment_otherComment_asAdmin(t *testing.T) {
	deleteCalled := false
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{
			FindByIDFn: func(_ context.Context, id uint) (*domainModel.TodoComment, error) {
				return fixedComment(id, 1, 5), nil // owned by user 5
			},
			DeleteFn: func(_ context.Context, _ uint) error {
				deleteCalled = true
				return nil
			},
		},
	)
	// callerID=99 (not owner), isAdmin=true
	if err := uc.DeleteComment(context.Background(), 1, 99, true); err != nil {
		t.Fatalf("expected no error for admin, got %v", err)
	}
	if !deleteCalled {
		t.Error("expected Delete to be called")
	}
}

func TestTodoOwned_DeleteComment_otherComment_notAdmin(t *testing.T) {
	uc := newOwnedUsecase(
		&repoMock.TodoOwnedRepositoryMock{},
		&repoMock.UserRepositoryMock{},
		&repoMock.TodoCommentRepositoryMock{
			FindByIDFn: func(_ context.Context, id uint) (*domainModel.TodoComment, error) {
				return fixedComment(id, 1, 5), nil // owned by user 5
			},
		},
	)
	// callerID=99 (not owner), isAdmin=false
	err := uc.DeleteComment(context.Background(), 1, 99, false)
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 403 {
		t.Errorf("expected 403 Forbidden, got %v", err)
	}
}
