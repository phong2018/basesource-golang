package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourname/go-clean-base/internal/domain/model"
	domainsvc "github.com/yourname/go-clean-base/internal/domain/service"
)

type jwtTokenService struct {
	secret           []byte
	accessTTLMinutes int
}

func NewJWTTokenService(secret string, accessTTLMinutes int) domainsvc.ITokenService {
	return &jwtTokenService{
		secret:           []byte(secret),
		accessTTLMinutes: accessTTLMinutes,
	}
}

func (s *jwtTokenService) GenerateAccessToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  fmt.Sprintf("%d", user.ID),
		"role": user.Role,
		"exp":  time.Now().Add(time.Duration(s.accessTTLMinutes) * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *jwtTokenService) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *jwtTokenService) ValidateAccessToken(tokenStr string) (*model.TokenClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, model.ErrTokenInvalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, model.ErrTokenInvalid
	}
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if sub == "" || role == "" {
		return nil, model.ErrTokenInvalid
	}
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return nil, model.ErrTokenInvalid
	}
	return &model.TokenClaims{UserID: userID, Role: role}, nil
}

func (s *jwtTokenService) HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
