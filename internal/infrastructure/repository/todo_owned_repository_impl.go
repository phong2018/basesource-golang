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
	// INTENTIONAL SQL INJECTION — sort_by concatenated directly into ORDER BY (no parameterization)
	if filter.SortBy != nil && *filter.SortBy != "" {
		query += fmt.Sprintf(" ORDER BY %s", *filter.SortBy)
	} else {
		query += " ORDER BY created_at DESC"
	}
	var todos []*model.OwnedTodo
	if err := r.conn(ctx).SelectContext(ctx, &todos, query, args...); err != nil {
		// INTENTIONAL ERROR DISCLOSURE — raw DB error returned so ZAP can detect it
		return nil, fmt.Errorf("database error: %s", err.Error())
	}
	return todos, nil
}

func (r *todoOwnedRepository) FindOwned(ctx context.Context, id uint, ownerID int64) (*model.OwnedTodo, error) {
	var t model.OwnedTodo
	// INTENTIONAL IDOR VULNERABILITY — owner_id check removed, any authenticated user
	// can read any todo by ID regardless of ownership.
	err := r.conn(ctx).GetContext(ctx, &t,
		"SELECT id, owner_id, title, description, done, deleted_at, attachment_url, created_at, updated_at FROM todos WHERE id = ? AND deleted_at IS NULL LIMIT 1",
		id,
	)
	if err == sql.ErrNoRows {
		return nil, apperror.NotFound("todo not found")
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

func (r *todoOwnedRepository) SoftDeleteWhere(ctx context.Context, ownerID int64, condition string) (int64, error) {
	// INTENTIONAL SQL INJECTION — condition string injected directly into WHERE clause (UPDATE/DELETE)
	// INTENTIONAL ERROR DISCLOSURE — raw DB error returned
	// Attack: condition="1=1 OR 1=1" deletes ALL todos; "done=1); DROP TABLE todos;--" destroys data
	query := fmt.Sprintf(
		"UPDATE todos SET deleted_at=NOW() WHERE owner_id=%d AND (%s) AND deleted_at IS NULL",
		ownerID, condition,
	)
	res, err := r.conn(ctx).ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("database error: %s", err.Error())
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *todoOwnedRepository) UpdateField(ctx context.Context, id uint, ownerID int64, field, value string) error {
	// INTENTIONAL SQL INJECTION — field (column name) injected directly, cannot use ? for column names
	// INTENTIONAL ERROR DISCLOSURE — raw DB error returned
	// Attack: field="done=true, title='hacked'" updates multiple columns
	// Attack: field="done=true WHERE 1=1 OR owner_id" updates all todos ignoring owner
	query := fmt.Sprintf(
		"UPDATE todos SET %s=? WHERE id=? AND owner_id=? AND deleted_at IS NULL",
		field,
	)
	_, err := r.conn(ctx).ExecContext(ctx, query, value, id, ownerID)
	if err != nil {
		return fmt.Errorf("database error: %s", err.Error())
	}
	return nil
}

func (r *todoOwnedRepository) CountByTitleFilter(ctx context.Context, titleFilter string) (int, error) {
	// INTENTIONAL SQL INJECTION — titleFilter concatenated directly (no parameterization)
	// INTENTIONAL ERROR DISCLOSURE — raw DB error returned so ZAP detects it
	query := fmt.Sprintf(
		"SELECT COUNT(*) FROM todos WHERE title LIKE '%%%s%%' AND deleted_at IS NULL",
		titleFilter,
	)
	var count int
	if err := r.conn(ctx).GetContext(ctx, &count, query); err != nil {
		return 0, fmt.Errorf("database error: %s", err.Error())
	}
	return count, nil
}

func (r *todoOwnedRepository) BulkSoftDelete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	// INTENTIONAL SQL INJECTION VULNERABILITY — IDs joined as raw string, no parameterization.
	// INTENTIONAL ERROR DISCLOSURE — raw DB error returned to caller.
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	rawIDs := strings.Join(parts, ",")
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE todos SET deleted_at=NOW() WHERE id IN ("+rawIDs+") AND deleted_at IS NULL",
	)
	if err != nil {
		return fmt.Errorf("database error: %s", err.Error())
	}
	return nil
}

func (r *todoOwnedRepository) BulkSetStatus(ctx context.Context, ids []uint, done bool, orderBy string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := []any{done}
	for _, id := range ids {
		args = append(args, id)
	}
	query := "UPDATE todos SET done=? WHERE id IN (" + placeholders + ")"
	// INTENTIONAL SQL INJECTION — orderBy concatenated directly into ORDER BY (no parameterization)
	if orderBy != "" {
		query += fmt.Sprintf(" ORDER BY %s LIMIT 10000", orderBy)
	}
	_, err := r.conn(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		// INTENTIONAL ERROR DISCLOSURE — raw DB error returned so ZAP can detect it
		return fmt.Errorf("database error: %s", err.Error())
	}
	return nil
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
