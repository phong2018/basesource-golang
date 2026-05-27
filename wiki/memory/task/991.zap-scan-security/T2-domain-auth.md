# T2 — Domain Layer: Auth Models & Interfaces

**Status:** [x] Done

## Depends on
- T1 (migration must exist first for context, though domain has no DB dependency)

## Scope constraint
Domain layer must never import `infrastructure` or `usecase`. Only stdlib and `pkg/apperror`.

## Files to create

### `internal/domain/model/user.go`
```go
package model

import "time"

// Entity: maps 1:1 to users table
type User struct {
    ID        int64     `db:"id"`
    Email     string    `db:"email"`
    Password  string    `db:"password"` // bcrypt hash
    Role      string    `db:"role"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}
```

### `internal/domain/model/refresh_token.go`
```go
package model

import "time"

// Entity: maps 1:1 to refresh_tokens table
type RefreshToken struct {
    ID        int64     `db:"id"`
    UserID    int64     `db:"user_id"`
    TokenHash string    `db:"token_hash"`
    ExpiresAt time.Time `db:"expires_at"`
    Revoked   bool      `db:"revoked"`
    CreatedAt time.Time `db:"created_at"`
}
```

### `internal/domain/model/token_claims.go`
```go
package model

// Value Object: JWT claims extracted from a validated access token.
// Lives in domain so ITokenService (domain/service) can reference it
// without importing infrastructure.
type TokenClaims struct {
    UserID int64
    Role   string
}
```

### `internal/domain/model/user_constant.go`
```go
package model

// Domain Rule Constants: User
const (
    RoleAdmin = "admin"
    RoleUser  = "user"
)
```

### `internal/domain/model/user_error.go`
```go
package model

import "github.com/yourname/go-clean-base/pkg/apperror"

var (
    ErrInvalidCredentials = apperror.Unauthorized("invalid email or password")
    ErrEmailTaken         = apperror.Conflict("email already registered")
    ErrTokenExpired       = apperror.Unauthorized("token expired")
    ErrTokenInvalid       = apperror.Unauthorized("token invalid")
)
```

### `internal/domain/repository/user_repository.go`
```go
package repository

import (
    "context"
    "time"

    "github.com/yourname/go-clean-base/internal/domain/model"
)

type IUserRepository interface {
    FindByEmail(ctx context.Context, email string) (*model.User, error)
    FindByID(ctx context.Context, id int64) (*model.User, error)
    Create(ctx context.Context, user *model.User) error
    SaveRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
    FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
    RevokeRefreshToken(ctx context.Context, tokenHash string) error
}
```

### `internal/domain/service/token_service.go`
```go
package service

import "github.com/yourname/go-clean-base/internal/domain/model"

type ITokenService interface {
    GenerateAccessToken(user *model.User) (string, error)
    GenerateRefreshToken() (string, error) // random opaque token
    ValidateAccessToken(token string) (*model.TokenClaims, error)
    HashToken(token string) string
}
```

## Verification

```bash
# No infrastructure or usecase imports in domain
grep -r "infrastructure" internal/domain/   # must be zero results
grep -r "usecase"        internal/domain/   # must be zero results

# Build passes
go build ./internal/domain/...
```

## Done when
- [ ] All 7 files created
- [ ] `go build ./internal/domain/...` passes
- [ ] Zero infrastructure/usecase imports in domain
