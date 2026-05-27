package usecase

import (
	"context"

	"github.com/yourname/go-clean-base/internal/usecase/dto"
)

type IAuthUsecase interface {
	Register(ctx context.Context, req dto.RegisterRequest) error
	Login(ctx context.Context, req dto.LoginRequest) (dto.AuthResponse, error)
	Refresh(ctx context.Context, req dto.RefreshRequest) (dto.AuthResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (dto.MeResponse, error)
}
