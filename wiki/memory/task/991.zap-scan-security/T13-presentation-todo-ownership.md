# T13 — Presentation Layer: Todo Ownership Handlers & Routes

**Status:** [x] Done

## Depends on
- T5 (JWTMiddleware, RoleMiddleware already in place)
- T11 (ITodoOwnedUsecase interface)

## Scope constraint
New route groups only — appended after existing routes. Nothing in the existing `todos` block is touched.

## Files to create

### `internal/presentation/http/handler/todo_owned_handler.go`

Read `user_id` and `role` from Echo context (set by `JWTMiddleware`):

```go
func NewTodoOwnedHandler(uc usecase.ITodoOwnedUsecase) *TodoOwnedHandler

func (h *TodoOwnedHandler) ListMine(c echo.Context) error        // GET  /my/todos
func (h *TodoOwnedHandler) GetMine(c echo.Context) error         // GET  /my/todos/:id
func (h *TodoOwnedHandler) CreateMine(c echo.Context) error      // POST /my/todos
func (h *TodoOwnedHandler) UpdateMine(c echo.Context) error      // PUT  /my/todos/:id
func (h *TodoOwnedHandler) DeleteMine(c echo.Context) error      // DELETE /my/todos/:id
func (h *TodoOwnedHandler) Share(c echo.Context) error           // POST /my/todos/:id/share
func (h *TodoOwnedHandler) RevokeShare(c echo.Context) error     // DELETE /my/todos/:id/share/:uid
func (h *TodoOwnedHandler) UploadAttachment(c echo.Context) error // POST /my/todos/:id/attachment
func (h *TodoOwnedHandler) DeleteAttachment(c echo.Context) error // DELETE /my/todos/:id/attachment
```

Extract caller identity helper (reuse across handlers):
```go
func callerID(c echo.Context) int64 {
    id, _ := c.Get(middleware.ContextKeyUserID).(int64)
    return id
}
func callerRole(c echo.Context) string {
    role, _ := c.Get(middleware.ContextKeyRole).(string)
    return role
}
```

### `internal/presentation/http/handler/todo_admin_handler.go`

```go
func NewTodoAdminHandler(uc usecase.ITodoOwnedUsecase) *TodoAdminHandler

func (h *TodoAdminHandler) BulkDelete(c echo.Context) error    // POST  /admin/todos/bulk-delete
func (h *TodoAdminHandler) BulkSetStatus(c echo.Context) error // PATCH /admin/todos/bulk-status
```

### `internal/presentation/http/handler/todo_comment_handler.go`

```go
func NewTodoCommentHandler(uc usecase.ITodoOwnedUsecase) *TodoCommentHandler

func (h *TodoCommentHandler) List(c echo.Context) error   // GET    /todos/:id/comments
func (h *TodoCommentHandler) Add(c echo.Context) error    // POST   /todos/:id/comments
func (h *TodoCommentHandler) Delete(c echo.Context) error // DELETE /todos/:id/comments/:cid
```

`Delete` passes `isAdmin = callerRole(c) == model.RoleAdmin` to the usecase.

### Route registration in `server.go`

Append after the auth routes added in T5:

```go
ownedHandler   := handler.NewTodoOwnedHandler(deps.TodoOwnedUsecase)
adminHandler   := handler.NewTodoAdminHandler(deps.TodoOwnedUsecase)
commentHandler := handler.NewTodoCommentHandler(deps.TodoOwnedUsecase)

// Owner-scoped todo routes
my := v1.Group("/my/todos", middleware.JWTMiddleware(deps.TokenService))
my.GET("",                   ownedHandler.ListMine)
my.GET("/:id",               ownedHandler.GetMine)
my.POST("",                  ownedHandler.CreateMine)
my.PUT("/:id",               ownedHandler.UpdateMine)
my.DELETE("/:id",            ownedHandler.DeleteMine)
my.POST("/:id/share",        ownedHandler.Share)
my.DELETE("/:id/share/:uid", ownedHandler.RevokeShare)
my.POST("/:id/attachment",   ownedHandler.UploadAttachment)
my.DELETE("/:id/attachment", ownedHandler.DeleteAttachment)

// Admin bulk operations (role guard on group)
admin := v1.Group("/admin/todos",
    middleware.JWTMiddleware(deps.TokenService),
    middleware.RoleMiddleware(model.RoleAdmin),
)
admin.POST("/bulk-delete",  adminHandler.BulkDelete)
admin.PATCH("/bulk-status", adminHandler.BulkSetStatus)

// Comments (any authenticated user)
comments := v1.Group("/todos/:id/comments", middleware.JWTMiddleware(deps.TokenService))
comments.GET("",       commentHandler.List)
comments.POST("",      commentHandler.Add)
comments.DELETE("/:cid", commentHandler.Delete)
```

## Verification

```bash
go build ./...

# Start server (requires T6 container wiring complete)
go run main.go api &

ADMIN_TOKEN=<from seed step>
USER_TOKEN=<from seed step>

# Owner CRUD
curl -s -X POST http://localhost:8080/api/v1/my/todos \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"My first todo"}' | jq .
# Expected: 201 with OwnedTodoOutput

curl -s http://localhost:8080/api/v1/my/todos \
  -H "Authorization: Bearer $USER_TOKEN" | jq .
# Expected: 200 array

# Unauthenticated access denied
curl -s http://localhost:8080/api/v1/my/todos | jq .
# Expected: 401

# User cannot use admin bulk-delete
curl -s -X POST http://localhost:8080/api/v1/admin/todos/bulk-delete \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1]}' | jq .
# Expected: 403

# Admin can use bulk-delete
curl -s -X POST http://localhost:8080/api/v1/admin/todos/bulk-delete \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[999]}' | jq .
# Expected: 200 or 404 (not 403)
```

## Done when
- [ ] All 3 handler files created
- [ ] Routes appended in `server.go`
- [ ] `go build ./...` passes
- [ ] `POST /my/todos` with valid token returns 201
- [ ] `GET /my/todos` without token returns 401
- [ ] `POST /admin/todos/bulk-delete` with user token returns 403
- [ ] `POST /admin/todos/bulk-delete` with admin token does not return 403
- [ ] `GET /todos` (existing, public) still returns 200 without token
