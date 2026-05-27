# T6 — Container Wiring

**Status:** [x] Done

## Depends on
- T3 (IAuthUsecase)
- T4 (user_repository_impl, jwt_token_service)
- T5 (Dependencies struct updated)
- T11 (ITodoOwnedUsecase — wire after T11 is done)
- T12 (todo_owned_repository_impl, todo_comment_repository_impl)
- T13 (TodoOwnedHandler, TodoAdminHandler, TodoCommentHandler)

> Wire T1–T8 first (auth only). Come back and add T9–T14 wiring once Part 1.5 tasks are done.

## File to modify

`container/container.go`

Add the following (all existing wiring stays untouched):

```go
// ── Auth infrastructure ──────────────────────────────────────────────────────
tokenSvc := token.NewJWTTokenService(cfg.JWTSecret, cfg.JWTAccessTTLMinutes)
userRepo := repository.NewUserRepository(db)

// ── Auth usecase ─────────────────────────────────────────────────────────────
authUsecase := usecase.NewAuthUsecase(userRepo, tokenSvc)

// ── Todo owned (Part 1.5) — add after T12 is complete ────────────────────────
todoOwnedRepo    := repository.NewTodoOwnedRepository(db)
todoCommentRepo  := repository.NewTodoCommentRepository(db)
todoOwnedUsecase := usecase.NewTodoOwnedUsecase(todoOwnedRepo, userRepo, todoCommentRepo)

// ── HTTP server dependencies ──────────────────────────────────────────────────
deps := http.Dependencies{
    TodoUsecase:      todoUsecase,       // existing — do not remove
    AuthUsecase:      authUsecase,
    TodoOwnedUsecase: todoOwnedUsecase,
    TokenService:     tokenSvc,
}
```

Also add config field reads (from T7):
```go
cfg.JWTSecret              // os.Getenv("JWT_SECRET")
cfg.JWTAccessTTLMinutes    // os.Getenv("JWT_ACCESS_TTL_MINUTES"), default 15
cfg.JWTRefreshTTLDays      // os.Getenv("JWT_REFRESH_TTL_DAYS"),   default 7
```

## Verification

```bash
go build ./...
# Must compile with no unused-import or undefined-symbol errors

go run main.go api
curl http://localhost:8080/health
# Expected: 200
```

## Done when
- [ ] `container.go` updated with all new wiring
- [ ] Config reads for JWT env vars added
- [ ] `go build ./...` passes with no errors
- [ ] Server starts and `/health` returns 200
