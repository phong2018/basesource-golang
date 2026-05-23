package usecase

import (
	"context"
	"log/slog"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	domainSvc "github.com/yourname/go-clean-base/internal/domain/service"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
	"github.com/yourname/go-clean-base/pkg/helper"
)

type todoUsecase struct {
	repo         domainRepo.ITodoRepository
	auditLogRepo domainRepo.IAuditLogRepository
	tx           ITransaction
	notifier     domainSvc.INotificationClient
}

func NewTodoUsecase(
	repo domainRepo.ITodoRepository,
	auditLogRepo domainRepo.IAuditLogRepository,
	tx ITransaction,
	notifier domainSvc.INotificationClient,
) ITodoUsecase {
	return &todoUsecase{repo: repo, auditLogRepo: auditLogRepo, tx: tx, notifier: notifier}
}

func (u *todoUsecase) GetByID(ctx context.Context, id uint) (*dto.TodoOutput, error) {
	todo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx, "GetByID failed", "error", err, "id", id)
		return nil, err
	}
	return mapToOutput(todo), nil
}

func (u *todoUsecase) List(ctx context.Context, input dto.ListTodoInput) ([]*dto.TodoOutput, error) {
	filter := domainModel.TodoFilter{
		Done:   input.Done,
		Search: input.Search,
	}
	page := domainModel.Pagination{
		Page:  input.Page,
		Limit: input.Limit,
	}
	todos, err := u.repo.List(ctx, filter, page)
	if err != nil {
		slog.ErrorContext(ctx, "List failed", "error", err)
		return nil, apperror.Internal(err)
	}
	out := make([]*dto.TodoOutput, len(todos))
	for i, t := range todos {
		out[i] = mapToOutput(t)
	}
	return out, nil
}

func (u *todoUsecase) Create(ctx context.Context, input dto.CreateTodoInput) (*dto.TodoOutput, error) {
	var created *domainModel.Todo

	err := u.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		todo := &domainModel.Todo{
			Title:       input.Title,
			Description: input.Description,
		}
		var err error
		created, err = u.repo.Create(ctx, todo)
		if err != nil {
			return err
		}
		return u.auditLogRepo.Create(ctx, &domainModel.AuditLog{
			Entity:   "todo",
			EntityID: created.ID,
			Action:   domainModel.AuditActionCreate,
		})
	})
	if err != nil {
		slog.ErrorContext(ctx, "Create failed", "error", err)
		return nil, apperror.Internal(err)
	}

	// notification runs after commit — non-fatal, no rollback needed
	n := &domainModel.Notification{
		To:      "admin@example.com",
		Subject: "New Todo Created",
		Body:    "Todo: " + created.Title,
	}
	if _, err := u.notifier.Send(ctx, n); err != nil {
		slog.ErrorContext(ctx, "notification send failed", "error", err)
	}
	return mapToOutput(created), nil
}

func (u *todoUsecase) Update(ctx context.Context, input dto.UpdateTodoInput) (*dto.TodoOutput, error) {
	existing, err := u.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	var updated *domainModel.Todo

	err = u.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		existing.Title = input.Title
		existing.Description = input.Description
		existing.Done = input.Done

		var err error
		updated, err = u.repo.Update(ctx, existing)
		if err != nil {
			return err
		}
		return u.auditLogRepo.Create(ctx, &domainModel.AuditLog{
			Entity:   "todo",
			EntityID: updated.ID,
			Action:   domainModel.AuditActionUpdate,
		})
	})
	if err != nil {
		slog.ErrorContext(ctx, "Update failed", "error", err, "id", input.ID)
		return nil, apperror.Internal(err)
	}
	return mapToOutput(updated), nil
}

func (u *todoUsecase) Delete(ctx context.Context, id uint) error {
	if _, err := u.repo.GetByID(ctx, id); err != nil {
		return err
	}

	err := u.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := u.repo.Delete(ctx, id); err != nil {
			return err
		}
		return u.auditLogRepo.Create(ctx, &domainModel.AuditLog{
			Entity:   "todo",
			EntityID: id,
			Action:   domainModel.AuditActionDelete,
		})
	})
	if err != nil {
		slog.ErrorContext(ctx, "Delete failed", "error", err, "id", id)
		return apperror.Internal(err)
	}
	return nil
}

func mapToOutput(t *domainModel.Todo) *dto.TodoOutput {
	return &dto.TodoOutput{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Done:        t.Done,
		CreatedAt:   helper.FormatTime(t.CreatedAt),
		UpdatedAt:   helper.FormatTime(t.UpdatedAt),
	}
}
