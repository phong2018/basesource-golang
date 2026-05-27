package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
	domainsvc "github.com/yourname/go-clean-base/internal/domain/service"
	"github.com/yourname/go-clean-base/pkg/apperror"
)

const (
	ContextKeyUserID = "user_id"
	ContextKeyRole   = "role"
)

func JWTMiddleware(tokenSvc domainsvc.ITokenService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				return apperror.Unauthorized("missing or malformed token")
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := tokenSvc.ValidateAccessToken(tokenStr)
			if err != nil {
				return apperror.Unauthorized("token invalid or expired")
			}
			c.Set(ContextKeyUserID, claims.UserID)
			c.Set(ContextKeyRole, claims.Role)
			return next(c)
		}
	}
}

func RoleMiddleware(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get(ContextKeyRole).(string)
			if _, ok := allowed[role]; !ok {
				return apperror.Forbidden("insufficient permissions")
			}
			return next(c)
		}
	}
}
