# T5 — Presentation Layer: Auth Middleware, Handler & Routes

**Status:** [x] Done

## Depends on
- T3 (IAuthUsecase interface)
- T4 (ITokenService, for middleware)

## Scope constraint
- Existing `/todos` routes in `server.go` are **not touched** — they remain public
- Existing middleware (`ErrorHandler`, `RequestLogger`) are **not removed**
- Only new code is appended

## Files to create

### `internal/presentation/http/middleware/auth.go`

```go
package middleware

import (
    "net/http"
    "strings"

    "github.com/labstack/echo/v4"
    "github.com/yourname/go-clean-base/internal/domain/service"
    "github.com/yourname/go-clean-base/pkg/apperror"
)

const (
    ContextKeyUserID = "user_id"
    ContextKeyRole   = "role"
)

// JWTMiddleware extracts and validates the Bearer token.
// Sets user_id (int64) and role (string) into Echo context.
func JWTMiddleware(tokenSvc service.ITokenService) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            header := c.Request().Header.Get("Authorization")
            if !strings.HasPrefix(header, "Bearer ") {
                return apperror.Unauthorized("missing or malformed token")
            }
            token := strings.TrimPrefix(header, "Bearer ")
            claims, err := tokenSvc.ValidateAccessToken(token)
            if err != nil {
                return apperror.Unauthorized("token invalid or expired")
            }
            c.Set(ContextKeyUserID, claims.UserID)
            c.Set(ContextKeyRole, claims.Role)
            return next(c)
        }
    }
}

// RoleMiddleware checks that the authenticated user holds one of the required roles.
// Must be used after JWTMiddleware.
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
```

### `internal/presentation/http/handler/auth_handler.go`

Handlers for auth routes and `/me`. Extract `user_id` from `c.Get(ContextKeyUserID)` for the Me handler.

Methods:
- `Register(c echo.Context) error` — bind+validate `RegisterRequest`, call usecase, return 201
- `Login(c echo.Context) error` — bind `LoginRequest`, call usecase, return 200 with `AuthResponse`
- `Refresh(c echo.Context) error` — bind `RefreshRequest`, call usecase, return 200
- `Logout(c echo.Context) error` — bind `RefreshRequest`, call usecase, return 204
- `Me(c echo.Context) error` — read `user_id` from context, call usecase, return 200

Constructor:
```go
func NewAuthHandler(uc usecase.IAuthUsecase) *AuthHandler
```

### `internal/presentation/http/server.go` — changes

Update `Dependencies` struct (add fields, keep existing `TodoUsecase`):
```go
type Dependencies struct {
    TodoUsecase      usecase.ITodoUsecase   // existing — do not remove
    AuthUsecase      usecase.IAuthUsecase
    TodoOwnedUsecase usecase.ITodoOwnedUsecase
    TokenService     service.ITokenService
}
```

Append to `NewServer` **after** the existing route block:
```go
// Security headers
e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
    XSSProtection:         "1; mode=block",
    ContentTypeNosniff:    "nosniff",
    XFrameOptions:         "SAMEORIGIN",
    HSTSMaxAge:            31536000,
    ContentSecurityPolicy: "default-src 'self'",
}))

// Auth routes (public)
authHandler := handler.NewAuthHandler(deps.AuthUsecase)
auth := v1.Group("/auth")
auth.POST("/register", authHandler.Register)
auth.POST("/login",    authHandler.Login)
auth.POST("/refresh",  authHandler.Refresh)
auth.POST("/logout",   authHandler.Logout)

// Current user profile (protected)
me := v1.Group("/me", middleware.JWTMiddleware(deps.TokenService))
me.GET("", authHandler.Me)
```

## Verification

```bash
go build ./...

# Start server and test public auth endpoints
go run main.go api &

curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"Test1234!"}' | jq .
# Expected: 201 with no body or success message

curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"Test1234!"}' | jq .
# Expected: 200 with access_token, refresh_token, expires_in

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"Test1234!"}' | jq -r '.access_token')

curl -s http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 with id, email, role

curl -s http://localhost:8080/api/v1/me | jq .
# Expected: 401

# Existing todos route still works without token
curl -s http://localhost:8080/api/v1/todos | jq .
# Expected: 200 (still public)
```

## Done when
- [ ] `middleware/auth.go` created with `JWTMiddleware` and `RoleMiddleware`
- [ ] `handler/auth_handler.go` created with all 5 methods
- [ ] `Dependencies` struct updated in `server.go`
- [ ] Auth and `/me` routes appended in `server.go`
- [ ] `go build ./...` passes
- [ ] `POST /auth/register` returns 201
- [ ] `POST /auth/login` returns 200 with tokens
- [ ] `GET /me` with valid token returns 200
- [ ] `GET /me` without token returns 401
- [ ] `GET /todos` still returns 200 without a token (not broken)
