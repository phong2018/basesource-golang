package service

import "github.com/yourname/go-clean-base/internal/domain/model"

type ITokenService interface {
	GenerateAccessToken(user *model.User) (string, error)
	GenerateRefreshToken() (string, error)
	ValidateAccessToken(token string) (*model.TokenClaims, error)
	HashToken(token string) string
}
