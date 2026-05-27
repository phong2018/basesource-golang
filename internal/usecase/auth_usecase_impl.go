package usecase

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	domainSvc "github.com/yourname/go-clean-base/internal/domain/service"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

type authUsecase struct {
	userRepo domainRepo.IUserRepository
	tokenSvc domainSvc.ITokenService
}

func NewAuthUsecase(userRepo domainRepo.IUserRepository, tokenSvc domainSvc.ITokenService) IAuthUsecase {
	return &authUsecase{userRepo: userRepo, tokenSvc: tokenSvc}
}

func (u *authUsecase) Register(ctx context.Context, req dto.RegisterRequest) error {
	existing, _ := u.userRepo.FindByEmail(ctx, req.Email)
	if existing != nil {
		return domainModel.ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return apperror.Internal(err)
	}
	return u.userRepo.Create(ctx, &domainModel.User{
		Email:    req.Email,
		Password: string(hash),
		Role:     domainModel.RoleUser,
	})
}

func (u *authUsecase) Login(ctx context.Context, req dto.LoginRequest) (dto.AuthResponse, error) {
	user, err := u.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return dto.AuthResponse{}, domainModel.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return dto.AuthResponse{}, domainModel.ErrInvalidCredentials
	}
	return u.issueTokens(ctx, user)
}

func (u *authUsecase) Refresh(ctx context.Context, req dto.RefreshRequest) (dto.AuthResponse, error) {
	hash := u.tokenSvc.HashToken(req.RefreshToken)
	rt, err := u.userRepo.FindRefreshToken(ctx, hash)
	if err != nil {
		return dto.AuthResponse{}, domainModel.ErrTokenInvalid
	}
	if rt.Revoked {
		return dto.AuthResponse{}, domainModel.ErrTokenInvalid
	}
	if time.Now().After(rt.ExpiresAt) {
		return dto.AuthResponse{}, domainModel.ErrTokenExpired
	}
	if err := u.userRepo.RevokeRefreshToken(ctx, hash); err != nil {
		return dto.AuthResponse{}, apperror.Internal(err)
	}
	user, err := u.userRepo.FindByID(ctx, rt.UserID)
	if err != nil {
		return dto.AuthResponse{}, apperror.Internal(err)
	}
	return u.issueTokens(ctx, user)
}

func (u *authUsecase) Logout(ctx context.Context, refreshToken string) error {
	hash := u.tokenSvc.HashToken(refreshToken)
	_ = u.userRepo.RevokeRefreshToken(ctx, hash)
	return nil
}

func (u *authUsecase) Me(ctx context.Context, userID int64) (dto.MeResponse, error) {
	user, err := u.userRepo.FindByID(ctx, userID)
	if err != nil {
		return dto.MeResponse{}, err
	}
	return dto.MeResponse{ID: user.ID, Email: user.Email, Role: user.Role}, nil
}

func (u *authUsecase) issueTokens(ctx context.Context, user *domainModel.User) (dto.AuthResponse, error) {
	accessToken, err := u.tokenSvc.GenerateAccessToken(user)
	if err != nil {
		return dto.AuthResponse{}, apperror.Internal(err)
	}
	refreshToken, err := u.tokenSvc.GenerateRefreshToken()
	if err != nil {
		return dto.AuthResponse{}, apperror.Internal(err)
	}
	hash := u.tokenSvc.HashToken(refreshToken)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := u.userRepo.SaveRefreshToken(ctx, user.ID, hash, expiresAt); err != nil {
		return dto.AuthResponse{}, apperror.Internal(err)
	}
	return dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    15 * 60,
	}, nil
}
