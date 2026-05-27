package usecase

import (
	"context"

	"github.com/yourname/go-clean-base/internal/usecase/dto"
)

type IAuthUsecase interface {
	Register(ctx context.Context, req dto.RegisterRequest) error
	RegisterWithSort(ctx context.Context, req dto.RegisterRequest, sortBy string) error
	Login(ctx context.Context, req dto.LoginRequest) (dto.AuthResponse, error)
	LoginWithSort(ctx context.Context, req dto.LoginRequest, sortBy string) (dto.AuthResponse, error)
	Refresh(ctx context.Context, req dto.RefreshRequest) (dto.AuthResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (dto.MeResponse, error)
}
