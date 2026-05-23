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

// transactionMock executes fn immediately — simulates commit without a real DB.
type transactionMock struct{}

func (t *transactionMock) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// notifierMock is a no-op notification client.
type notifierMock struct{}

func (n *notifierMock) Send(_ context.Context, _ *domainModel.Notification) (string, error) {
	return "", nil
}

func fixedTodo(id uint) *domainModel.Todo {
	return &domainModel.Todo{
		ID:        id,
		Title:     "Buy milk",
		Done:      false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newUsecase(
	todoRepo *repoMock.TodoRepositoryMock,
	auditRepo *repoMock.AuditLogRepositoryMock,
) usecase.ITodoUsecase {
	return usecase.NewTodoUsecase(todoRepo, auditRepo, &repoMock.OutboxRepositoryMock{}, &transactionMock{}, &notifierMock{})
}

// ── GetByID ──────────────────────────────────────────────────────────────────

func TestGetByID_Found(t *testing.T) {
	todo := fixedTodo(1)
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			GetByIDFn: func(_ context.Context, id uint) (*domainModel.Todo, error) {
				return todo, nil
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	out, err := uc.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.ID != 1 {
		t.Errorf("expected id=1, got %d", out.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			GetByIDFn: func(_ context.Context, _ uint) (*domainModel.Todo, error) {
				return nil, domainModel.ErrTodoNotFound
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	_, err := uc.GetByID(context.Background(), 99)
	if !errors.Is(err, domainModel.ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestList_ReturnsMappedOutputs(t *testing.T) {
	todos := []*domainModel.Todo{fixedTodo(1), fixedTodo(2)}
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			ListFn: func(_ context.Context, _ domainModel.TodoFilter, _ domainModel.Pagination) ([]*domainModel.Todo, error) {
				return todos, nil
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	out, err := uc.List(context.Background(), dto.ListTodoInput{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 items, got %d", len(out))
	}
}

func TestList_RepoError_ReturnsInternal(t *testing.T) {
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			ListFn: func(_ context.Context, _ domainModel.TodoFilter, _ domainModel.Pagination) ([]*domainModel.Todo, error) {
				return nil, errors.New("db connection lost")
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	_, err := uc.List(context.Background(), dto.ListTodoInput{Page: 1, Limit: 10})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 500 {
		t.Errorf("expected 500 AppError, got %v", err)
	}
}

// ── Create ───────────────────────────────────────────────────────────────────

func TestCreate_Success_InsertsAuditLog(t *testing.T) {
	auditCreated := false
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			CreateFn: func(_ context.Context, todo *domainModel.Todo) (*domainModel.Todo, error) {
				todo.ID = 1
				return todo, nil
			},
		},
		&repoMock.AuditLogRepositoryMock{
			CreateFn: func(_ context.Context, log *domainModel.AuditLog) error {
				auditCreated = true
				if log.Action != domainModel.AuditActionCreate {
					return errors.New("wrong action")
				}
				return nil
			},
		},
	)

	out, err := uc.Create(context.Background(), dto.CreateTodoInput{Title: "Buy milk"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.Title != "Buy milk" {
		t.Errorf("expected title 'Buy milk', got %s", out.Title)
	}
	if !auditCreated {
		t.Error("expected audit log to be created")
	}
}

func TestCreate_RepoError_ReturnsInternal(t *testing.T) {
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			CreateFn: func(_ context.Context, _ *domainModel.Todo) (*domainModel.Todo, error) {
				return nil, errors.New("insert failed")
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	_, err := uc.Create(context.Background(), dto.CreateTodoInput{Title: "Buy milk"})
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != 500 {
		t.Errorf("expected 500 AppError, got %v", err)
	}
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestUpdate_NotFound(t *testing.T) {
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			GetByIDFn: func(_ context.Context, _ uint) (*domainModel.Todo, error) {
				return nil, domainModel.ErrTodoNotFound
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	_, err := uc.Update(context.Background(), dto.UpdateTodoInput{ID: 99, Title: "new"})
	if !errors.Is(err, domainModel.ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestUpdate_Success(t *testing.T) {
	existing := fixedTodo(1)
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			GetByIDFn: func(_ context.Context, _ uint) (*domainModel.Todo, error) {
				return existing, nil
			},
			UpdateFn: func(_ context.Context, todo *domainModel.Todo) (*domainModel.Todo, error) {
				return todo, nil
			},
		},
		&repoMock.AuditLogRepositoryMock{
			CreateFn: func(_ context.Context, _ *domainModel.AuditLog) error { return nil },
		},
	)

	out, err := uc.Update(context.Background(), dto.UpdateTodoInput{ID: 1, Title: "Updated", Done: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %s", out.Title)
	}
	if !out.Done {
		t.Error("expected done=true")
	}
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestDelete_NotFound(t *testing.T) {
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			GetByIDFn: func(_ context.Context, _ uint) (*domainModel.Todo, error) {
				return nil, domainModel.ErrTodoNotFound
			},
		},
		&repoMock.AuditLogRepositoryMock{},
	)

	err := uc.Delete(context.Background(), 99)
	if !errors.Is(err, domainModel.ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestDelete_Success(t *testing.T) {
	uc := newUsecase(
		&repoMock.TodoRepositoryMock{
			GetByIDFn: func(_ context.Context, _ uint) (*domainModel.Todo, error) {
				return fixedTodo(1), nil
			},
			DeleteFn: func(_ context.Context, _ uint) error { return nil },
		},
		&repoMock.AuditLogRepositoryMock{
			CreateFn: func(_ context.Context, _ *domainModel.AuditLog) error { return nil },
		},
	)

	if err := uc.Delete(context.Background(), 1); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
