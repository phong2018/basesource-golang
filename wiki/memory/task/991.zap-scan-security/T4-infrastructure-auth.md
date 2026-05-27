# T4 — Infrastructure Layer: User Repository & JWT Token Service

**Status:** [x] Done

## Depends on
- T2 (domain interfaces to implement)
- T1 (tables must exist to run integration checks)

## Files to create

### `internal/infrastructure/repository/user_repository_impl.go`

Implement `domain/repository.IUserRepository` using `sqlx` raw SQL.

Key queries:
```sql
-- FindByEmail
SELECT id, email, password, role, created_at, updated_at
FROM users WHERE email = ? LIMIT 1

-- FindByID
SELECT id, email, password, role, created_at, updated_at
FROM users WHERE id = ? LIMIT 1

-- Create
INSERT INTO users (email, password, role, created_at, updated_at)
VALUES (:email, :password, :role, NOW(), NOW())

-- SaveRefreshToken
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
VALUES (:user_id, :token_hash, :expires_at, NOW())

-- FindRefreshToken
SELECT id, user_id, token_hash, expires_at, revoked, created_at
FROM refresh_tokens WHERE token_hash = ? LIMIT 1

-- RevokeRefreshToken
UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?
```

Constructor:
```go
func NewUserRepository(db *sqlx.DB) domain.IUserRepository
```

### `internal/infrastructure/token/jwt_token_service.go`

Implement `domain/service.ITokenService` using `golang-jwt/jwt/v5`.

- `GenerateAccessToken` — HS256 JWT, claims: `sub` (userID as string), `role`, `exp` (now + `JWT_ACCESS_TTL_MINUTES`)
- `GenerateRefreshToken` — `crypto/rand` 32 bytes, hex-encoded; caller stores the SHA-256 hash
- `ValidateAccessToken` — parse and verify signature + expiry, return `*model.TokenClaims`
- `HashToken` — `sha256.Sum256`, return hex string

Constructor:
```go
func NewJWTTokenService(secret string, accessTTLMinutes int) domain.ITokenService
```

## New dependency to add

```bash
go get github.com/golang-jwt/jwt/v5
```

## Verification

```bash
# Build passes
go build ./internal/infrastructure/...

# Dependency rule: infra must not import usecase or presentation
grep -r "usecase"      internal/infrastructure/  # must be zero results
grep -r "presentation" internal/infrastructure/  # must be zero results
```

## Done when
- [ ] `user_repository_impl.go` created, implements all 6 interface methods
- [ ] `jwt_token_service.go` created, implements all 4 interface methods
- [ ] `go get github.com/golang-jwt/jwt/v5` added to go.mod
- [ ] `go build ./internal/infrastructure/...` passes
- [ ] Zero usecase/presentation imports in infrastructure
