package repository

import (
	"context"
	"time"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type IUserRepository interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByEmailSorted(ctx context.Context, email, sortBy string) (*model.User, error)
	FindAllSorted(ctx context.Context, sortBy string) ([]*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	SaveRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}
