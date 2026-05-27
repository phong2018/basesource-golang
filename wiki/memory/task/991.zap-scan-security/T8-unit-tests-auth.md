# T8 — Unit Tests: Auth Usecase & JWT Middleware

**Status:** [x] Done

## Depends on
- T3 (auth_usecase_impl)
- T5 (JWTMiddleware, RoleMiddleware)

## Files to create

### `internal/usecase/auth_usecase_impl_test.go`

Use mocks for `IUserRepository` and `ITokenService` (follow the existing mock pattern in `internal/domain/repository/mock/`).

| Test case | Input | Expected |
|---|---|---|
| `Register_ok` | new email, valid password | no error, `Create` called once |
| `Register_duplicateEmail` | email already exists | `ErrEmailTaken` |
| `Login_ok` | correct email+password | returns `AuthResponse` with tokens |
| `Login_wrongPassword` | correct email, wrong password | `ErrInvalidCredentials` |
| `Login_unknownEmail` | unknown email | `ErrInvalidCredentials` (same error, no user enumeration) |
| `Refresh_ok` | valid non-revoked token hash | returns new `AuthResponse`, old token revoked |
| `Refresh_revoked` | revoked token | `ErrTokenInvalid` |
| `Refresh_expired` | expired token | `ErrTokenExpired` |
| `Logout_ok` | valid refresh token | no error |

### `internal/presentation/http/middleware/auth_test.go`

Use `httptest` and Echo.

| Test case | Setup | Expected HTTP status |
|---|---|---|
| `NoAuthHeader` | no `Authorization` header | 401 |
| `MalformedHeader` | `Authorization: Token abc` (not Bearer) | 401 |
| `InvalidToken` | `Bearer invalid.token.here` | 401 |
| `ExpiredToken` | `Bearer <expired JWT>` | 401 |
| `ValidToken` | `Bearer <valid JWT>` | passes through (200) |
| `RoleMiddleware_allowed` | role=admin, requires admin | passes (200) |
| `RoleMiddleware_forbidden` | role=user, requires admin | 403 |

## Verification

```bash
go test ./internal/usecase/... -run TestAuth -v
go test ./internal/presentation/http/middleware/... -v

# All tests pass
go test ./internal/usecase/... ./internal/presentation/http/middleware/...
# Expected: ok, no failures
```

## Done when
- [ ] `auth_usecase_impl_test.go` covers all 9 cases
- [ ] `auth_test.go` covers all 7 middleware cases
- [ ] `go test ./internal/usecase/... ./internal/presentation/http/middleware/...` passes with no failures
