# T3 — Usecase Layer: Auth

**Status:** [ ] Not started

## Depends on
- T2 (domain models and interfaces must exist)

## Scope constraint
Usecase imports only `domain`. Must never import `infrastructure` or `presentation`.

## Files to create

### `internal/usecase/dto/auth_dto.go`
```go
package dto

type RegisterRequest struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequest struct {
    RefreshToken string `json:"refresh_token" validate:"required"`
}

type MeResponse struct {
    ID    int64  `json:"id"`
    Email string `json:"email"`
    Role  string `json:"role"`
}
```

### `internal/usecase/auth_usecase.go`
```go
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
```

### `internal/usecase/auth_usecase_impl.go`

Implement `IAuthUsecase`. Constructor:
```go
func NewAuthUsecase(
    userRepo domainrepo.IUserRepository,
    tokenSvc domainsvc.ITokenService,
) IAuthUsecase
```

Method behaviour:
- `Register` — hash password with `bcrypt.GenerateFromPassword(cost=12)`, check `FindByEmail` returns not-found before creating, return `ErrEmailTaken` if duplicate
- `Login` — load user by email, compare hash with `bcrypt.CompareHashAndPassword`, generate access+refresh tokens, call `SaveRefreshToken` with `SHA-256(refreshToken)` hash and 7-day expiry
- `Refresh` — call `FindRefreshToken` by hash, check `Revoked` and `ExpiresAt`, call `RevokeRefreshToken`, issue new pair
- `Logout` — call `RevokeRefreshToken` by hash; ignore not-found
- `Me` — call `FindByID`, map to `MeResponse`

## Verification

```bash
# No infrastructure imports in usecase
grep -r "infrastructure" internal/usecase/  # must be zero results

# Build passes
go build ./internal/usecase/...
```

## Done when
- [ ] `internal/usecase/dto/auth_dto.go` created with all DTOs and validate tags
- [ ] `internal/usecase/auth_usecase.go` interface created
- [ ] `internal/usecase/auth_usecase_impl.go` implementation created
- [ ] `go build ./internal/usecase/...` passes
- [ ] Zero infrastructure imports in usecase
