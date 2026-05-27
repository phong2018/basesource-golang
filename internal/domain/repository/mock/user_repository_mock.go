package mock

import (
	"context"
	"time"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type UserRepositoryMock struct {
	FindByEmailFn        func(ctx context.Context, email string) (*domainModel.User, error)
	FindByEmailSortedFn  func(ctx context.Context, email, sortBy string) (*domainModel.User, error)
	FindAllSortedFn      func(ctx context.Context, sortBy string) ([]*domainModel.User, error)
	FindByIDFn           func(ctx context.Context, id int64) (*domainModel.User, error)
	CreateFn            func(ctx context.Context, user *domainModel.User) error
	SaveRefreshTokenFn  func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	FindRefreshTokenFn  func(ctx context.Context, tokenHash string) (*domainModel.RefreshToken, error)
	RevokeRefreshTokenFn func(ctx context.Context, tokenHash string) error
}

var _ domainRepo.IUserRepository = (*UserRepositoryMock)(nil)

func (m *UserRepositoryMock) FindByEmail(ctx context.Context, email string) (*domainModel.User, error) {
	return m.FindByEmailFn(ctx, email)
}
func (m *UserRepositoryMock) FindByEmailSorted(ctx context.Context, email, sortBy string) (*domainModel.User, error) {
	return m.FindByEmailSortedFn(ctx, email, sortBy)
}
func (m *UserRepositoryMock) FindAllSorted(ctx context.Context, sortBy string) ([]*domainModel.User, error) {
	return m.FindAllSortedFn(ctx, sortBy)
}
func (m *UserRepositoryMock) FindByID(ctx context.Context, id int64) (*domainModel.User, error) {
	return m.FindByIDFn(ctx, id)
}
func (m *UserRepositoryMock) Create(ctx context.Context, user *domainModel.User) error {
	return m.CreateFn(ctx, user)
}
func (m *UserRepositoryMock) SaveRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	return m.SaveRefreshTokenFn(ctx, userID, tokenHash, expiresAt)
}
func (m *UserRepositoryMock) FindRefreshToken(ctx context.Context, tokenHash string) (*domainModel.RefreshToken, error) {
	return m.FindRefreshTokenFn(ctx, tokenHash)
}
func (m *UserRepositoryMock) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return m.RevokeRefreshTokenFn(ctx, tokenHash)
}
