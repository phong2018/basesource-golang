package repository

import (
	"context"
	"database/sql"
	"fmt"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

type todoRepository struct {
	baseRepository
}

func NewTodoRepository(db *database.Client) domainRepo.ITodoRepository {
	return &todoRepository{baseRepository{db: db}}
}

func (r *todoRepository) GetByID(ctx context.Context, id uint) (*domainModel.Todo, error) {
	var todo domainModel.Todo
	err := r.conn(ctx).GetContext(ctx, &todo, "SELECT id, title, description, done, created_at, updated_at FROM todos WHERE id = ?", id)
	if err == sql.ErrNoRows {
		return nil, domainModel.ErrTodoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("GetByID: %w", err)
	}
	return &todo, nil
}

func (r *todoRepository) List(ctx context.Context, filter domainModel.TodoFilter, page domainModel.Pagination) ([]*domainModel.Todo, error) {
	query := "SELECT id, title, description, done, created_at, updated_at FROM todos WHERE 1=1"
	args := []any{}

	if filter.Done != nil {
		query += " AND done = ?"
		args = append(args, *filter.Done)
	}
	if filter.Search != nil && *filter.Search != "" {
		query += " AND title LIKE ?"
		args = append(args, "%"+*filter.Search+"%")
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, page.Limit, page.Offset())

	var todos []*domainModel.Todo
	if err := r.conn(ctx).SelectContext(ctx, &todos, query, args...); err != nil {
		return nil, fmt.Errorf("List: %w", err)
	}
	return todos, nil
}

func (r *todoRepository) Create(ctx context.Context, todo *domainModel.Todo) (*domainModel.Todo, error) {
	res, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO todos (title, description, done) VALUES (?, ?, ?)",
		todo.Title, todo.Description, todo.Done,
	)
	if err != nil {
		return nil, fmt.Errorf("Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("LastInsertId: %w", err)
	}
	return r.GetByID(ctx, uint(id))
}

func (r *todoRepository) Update(ctx context.Context, todo *domainModel.Todo) (*domainModel.Todo, error) {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET title=?, description=?, done=? WHERE id=?",
		todo.Title, todo.Description, todo.Done, todo.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("Update: %w", err)
	}
	return r.GetByID(ctx, todo.ID)
}

func (r *todoRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM todos WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("Delete: %w", err)
	}
	return nil
}
