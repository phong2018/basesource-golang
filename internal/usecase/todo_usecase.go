package usecase

import (
	"context"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
)

type ITodoUsecase interface {
	GetByID(ctx context.Context, id uint) (*dto.TodoOutput, error)
	List(ctx context.Context, input dto.ListTodoInput) ([]*dto.TodoOutput, error)
	Create(ctx context.Context, input dto.CreateTodoInput) (*dto.TodoOutput, error)
	Update(ctx context.Context, input dto.UpdateTodoInput) (*dto.TodoOutput, error)
	Delete(ctx context.Context, id uint) error
}
