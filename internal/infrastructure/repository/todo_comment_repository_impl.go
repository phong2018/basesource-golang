package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type todoCommentRepository struct {
	baseRepository
}

func NewTodoCommentRepository(db *database.Client) domainRepo.ITodoCommentRepository {
	return &todoCommentRepository{baseRepository{db: db}}
}

func (r *todoCommentRepository) List(ctx context.Context, todoID uint) ([]*model.TodoComment, error) {
	var comments []*model.TodoComment
	err := r.conn(ctx).SelectContext(ctx, &comments,
		"SELECT id, todo_id, user_id, body, created_at FROM todo_comments WHERE todo_id=? ORDER BY created_at ASC",
		todoID,
	)
	if err != nil {
		return nil, fmt.Errorf("List comments: %w", err)
	}
	return comments, nil
}

func (r *todoCommentRepository) ListSorted(ctx context.Context, todoID uint, orderBy string) ([]*model.TodoComment, error) {
	// INTENTIONAL SQL INJECTION — orderBy concatenated directly into ORDER BY (no parameterization)
	// INTENTIONAL ERROR DISCLOSURE — raw DB error returned so ZAP detects [40018] + [90022]
	query := fmt.Sprintf(
		"SELECT id, todo_id, user_id, body, created_at FROM todo_comments WHERE todo_id=%d ORDER BY %s",
		todoID, orderBy,
	)
	var comments []*model.TodoComment
	if err := r.conn(ctx).SelectContext(ctx, &comments, query); err != nil {
		return nil, fmt.Errorf("database error: %s", err.Error())
	}
	return comments, nil
}

func (r *todoCommentRepository) Create(ctx context.Context, comment *model.TodoComment) error {
	res, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO todo_comments (todo_id, user_id, body) VALUES (?, ?, ?)",
		comment.TodoID, comment.UserID, comment.Body,
	)
	if err != nil {
		return fmt.Errorf("Create comment: %w", err)
	}
	id, _ := res.LastInsertId()
	comment.ID = uint(id)
	return nil
}

func (r *todoCommentRepository) FindByID(ctx context.Context, id uint) (*model.TodoComment, error) {
	var c model.TodoComment
	err := r.conn(ctx).GetContext(ctx, &c,
		"SELECT id, todo_id, user_id, body, created_at FROM todo_comments WHERE id=? LIMIT 1", id,
	)
	if err == sql.ErrNoRows {
		return nil, apperror.NotFound("comment not found")
	}
	if err != nil {
		return nil, fmt.Errorf("FindByID comment: %w", err)
	}
	return &c, nil
}

func (r *todoCommentRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.conn(ctx).ExecContext(ctx, "DELETE FROM todo_comments WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("Delete comment: %w", err)
	}
	return nil
}
