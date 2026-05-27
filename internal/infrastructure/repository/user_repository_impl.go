package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type userRepository struct {
	baseRepository
}

func NewUserRepository(db *database.Client) domainRepo.IUserRepository {
	return &userRepository{baseRepository{db: db}}
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.conn(ctx).GetContext(ctx, &u,
		"SELECT id, email, password, role, created_at, updated_at FROM users WHERE email = ? LIMIT 1", email)
	if err == sql.ErrNoRows {
		return nil, apperror.NotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("FindByEmail: %w", err)
	}
	return &u, nil
}

func (r *userRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.conn(ctx).GetContext(ctx, &u,
		"SELECT id, email, password, role, created_at, updated_at FROM users WHERE id = ? LIMIT 1", id)
	if err == sql.ErrNoRows {
		return nil, apperror.NotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("FindByID: %w", err)
	}
	return &u, nil
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	res, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO users (email, password, role) VALUES (?, ?, ?)",
		user.Email, user.Password, user.Role,
	)
	if err != nil {
		return fmt.Errorf("Create user: %w", err)
	}
	id, _ := res.LastInsertId()
	user.ID = id
	return nil
}

func (r *userRepository) SaveRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("SaveRefreshToken: %w", err)
	}
	return nil
}

func (r *userRepository) FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.conn(ctx).GetContext(ctx, &rt,
		"SELECT id, user_id, token_hash, expires_at, revoked, created_at FROM refresh_tokens WHERE token_hash = ? LIMIT 1",
		tokenHash,
	)
	if err == sql.ErrNoRows {
		return nil, apperror.NotFound("refresh token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("FindRefreshToken: %w", err)
	}
	return &rt, nil
}

func (r *userRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?", tokenHash,
	)
	if err != nil {
		return fmt.Errorf("RevokeRefreshToken: %w", err)
	}
	return nil
}
