package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type todoOwnedRepository struct {
	baseRepository
}

func NewTodoOwnedRepository(db *database.Client) domainRepo.ITodoOwnedRepository {
	return &todoOwnedRepository{baseRepository{db: db}}
}

func (r *todoOwnedRepository) ListByOwner(ctx context.Context, ownerID int64, filter model.TodoFilter) ([]*model.OwnedTodo, error) {
	query := "SELECT id, owner_id, title, description, done, deleted_at, attachment_url, created_at, updated_at FROM todos WHERE owner_id = ? AND deleted_at IS NULL"
	args := []any{ownerID}
	if filter.Done != nil {
		query += " AND done = ?"
		args = append(args, *filter.Done)
	}
	if filter.Search != nil && *filter.Search != "" {
		query += " AND title LIKE ?"
		args = append(args, "%"+*filter.Search+"%")
	}
	query += " ORDER BY created_at DESC"
	var todos []*model.OwnedTodo
	if err := r.conn(ctx).SelectContext(ctx, &todos, query, args...); err != nil {
		return nil, fmt.Errorf("ListByOwner: %w", err)
	}
	return todos, nil
}

func (r *todoOwnedRepository) FindOwned(ctx context.Context, id uint, ownerID int64) (*model.OwnedTodo, error) {
	var t model.OwnedTodo
	err := r.conn(ctx).GetContext(ctx, &t,
		"SELECT id, owner_id, title, description, done, deleted_at, attachment_url, created_at, updated_at FROM todos WHERE id = ? AND owner_id = ? AND deleted_at IS NULL LIMIT 1",
		id, ownerID,
	)
	if err == sql.ErrNoRows {
		return nil, apperror.Forbidden("todo not found or access denied")
	}
	if err != nil {
		return nil, fmt.Errorf("FindOwned: %w", err)
	}
	return &t, nil
}

func (r *todoOwnedRepository) CreateOwned(ctx context.Context, todo *model.OwnedTodo) error {
	res, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO todos (owner_id, title, description, done) VALUES (?, ?, ?, ?)",
		todo.OwnerID, todo.Title, todo.Description, todo.Done,
	)
	if err != nil {
		return fmt.Errorf("CreateOwned: %w", err)
	}
	id, _ := res.LastInsertId()
	todo.ID = uint(id)
	return nil
}

func (r *todoOwnedRepository) UpdateOwned(ctx context.Context, todo *model.OwnedTodo) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET title=?, description=?, done=? WHERE id=? AND owner_id=? AND deleted_at IS NULL",
		todo.Title, todo.Description, todo.Done, todo.ID, todo.OwnerID,
	)
	if err != nil {
		return fmt.Errorf("UpdateOwned: %w", err)
	}
	return nil
}

func (r *todoOwnedRepository) SoftDeleteOwned(ctx context.Context, id uint, ownerID int64) error {
	res, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET deleted_at=NOW() WHERE id=? AND owner_id=? AND deleted_at IS NULL",
		id, ownerID,
	)
	if err != nil {
		return fmt.Errorf("SoftDeleteOwned: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperror.Forbidden("todo not found or access denied")
	}
	return nil
}

func (r *todoOwnedRepository) BulkSoftDelete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET deleted_at=NOW() WHERE id IN ("+placeholders+") AND deleted_at IS NULL",
		args...,
	)
	return err
}

func (r *todoOwnedRepository) BulkSetStatus(ctx context.Context, ids []uint, done bool) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := []any{done}
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET done=? WHERE id IN ("+placeholders+")",
		args...,
	)
	return err
}

func (r *todoOwnedRepository) Share(ctx context.Context, todoID uint, targetUserID int64) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"INSERT IGNORE INTO todo_shares (todo_id, user_id) VALUES (?, ?)",
		todoID, targetUserID,
	)
	if err != nil {
		return fmt.Errorf("Share: %w", err)
	}
	return nil
}

func (r *todoOwnedRepository) RevokeShare(ctx context.Context, todoID uint, targetUserID int64) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"DELETE FROM todo_shares WHERE todo_id=? AND user_id=?",
		todoID, targetUserID,
	)
	if err != nil {
		return fmt.Errorf("RevokeShare: %w", err)
	}
	return nil
}

func (r *todoOwnedRepository) UpdateAttachment(ctx context.Context, id uint, ownerID int64, url *string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET attachment_url=? WHERE id=? AND owner_id=?",
		url, id, ownerID,
	)
	if err != nil {
		return fmt.Errorf("UpdateAttachment: %w", err)
	}
	return nil
}
