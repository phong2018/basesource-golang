package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
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
	outboxRepo   domainRepo.IOutboxRepository
	tx           ITransaction
	notifier     domainSvc.INotificationClient
}

func NewTodoUsecase(
	repo         domainRepo.ITodoRepository,
	auditLogRepo domainRepo.IAuditLogRepository,
	outboxRepo   domainRepo.IOutboxRepository,
	tx           ITransaction,
	notifier     domainSvc.INotificationClient,
) ITodoUsecase {
	return &todoUsecase{
		repo:         repo,
		auditLogRepo: auditLogRepo,
		outboxRepo:   outboxRepo,
		tx:           tx,
		notifier:     notifier,
	}
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
		SortBy: input.SortBy,
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
		if err := u.auditLogRepo.Create(ctx, &domainModel.AuditLog{
			Entity:   "todo",
			EntityID: created.ID,
			Action:   domainModel.AuditActionCreate,
		}); err != nil {
			return err
		}
		return u.outboxRepo.CreateEventWithDeliveries(ctx,
				buildOutboxEvent(domainModel.EventTypeTodoCreated, "todo", fmt.Sprint(created.ID), created),
				[]string{domainModel.OutboxDestinationKafka, domainModel.OutboxDestinationRabbitMQ},
			)
	})
	if err != nil {
		slog.ErrorContext(ctx, "Create failed", "error", err)
		return nil, apperror.Internal(err)
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
		if err := u.auditLogRepo.Create(ctx, &domainModel.AuditLog{
			Entity:   "todo",
			EntityID: updated.ID,
			Action:   domainModel.AuditActionUpdate,
		}); err != nil {
			return err
		}
		return u.outboxRepo.CreateEventWithDeliveries(ctx,
				buildOutboxEvent(domainModel.EventTypeTodoUpdated, "todo", fmt.Sprint(updated.ID), updated),
				[]string{domainModel.OutboxDestinationKafka, domainModel.OutboxDestinationRabbitMQ},
			)
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
		if err := u.auditLogRepo.Create(ctx, &domainModel.AuditLog{
			Entity:   "todo",
			EntityID: id,
			Action:   domainModel.AuditActionDelete,
		}); err != nil {
			return err
		}
		return u.outboxRepo.CreateEventWithDeliveries(ctx,
				buildOutboxEvent(domainModel.EventTypeTodoDeleted, "todo", fmt.Sprint(id), map[string]any{"id": id}),
				[]string{domainModel.OutboxDestinationKafka, domainModel.OutboxDestinationRabbitMQ},
			)
	})
	if err != nil {
		slog.ErrorContext(ctx, "Delete failed", "error", err, "id", id)
		return apperror.Internal(err)
	}
	return nil
}

func buildOutboxEvent(eventType, aggregateType, aggregateID string, payload any) *domainModel.OutboxEvent {
	body, _ := json.Marshal(payload)
	return &domainModel.OutboxEvent{
		EventID:       uuid.NewString(),
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       string(body),
	}
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
