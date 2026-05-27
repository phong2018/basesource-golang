package mock

import (
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainSvc "github.com/yourname/go-clean-base/internal/domain/service"
)

type TokenServiceMock struct {
	GenerateAccessTokenFn  func(user *domainModel.User) (string, error)
	GenerateRefreshTokenFn func() (string, error)
	ValidateAccessTokenFn  func(token string) (*domainModel.TokenClaims, error)
	HashTokenFn            func(token string) string
}

var _ domainSvc.ITokenService = (*TokenServiceMock)(nil)

func (m *TokenServiceMock) GenerateAccessToken(user *domainModel.User) (string, error) {
	return m.GenerateAccessTokenFn(user)
}
func (m *TokenServiceMock) GenerateRefreshToken() (string, error) {
	return m.GenerateRefreshTokenFn()
}
func (m *TokenServiceMock) ValidateAccessToken(token string) (*domainModel.TokenClaims, error) {
	return m.ValidateAccessTokenFn(token)
}
func (m *TokenServiceMock) HashToken(token string) string {
	return m.HashTokenFn(token)
}
