# Coding Conventions — Go Base Source

---

## 1. Naming

| Concern | Convention | Example |
|---|---|---|
| Interface | Prefix `I` | `ITodoUsecase`, `ITodoRepository`, `INotificationClient` |
| Constructor | `NewXxx(deps...) IXxx` — returns interface, not struct | `NewTodoUsecase(repo, notifier) ITodoUsecase` |
| Implementation file | `xxx_impl.go` | `todo_usecase_impl.go`, `todo_repository_impl.go` |
| File names | `snake_case.go` | `todo_filter.go`, `notification_client.go` |
| Constants file | `xxx_constant.go` | `todo_constant.go` |
| Mock file | `xxx_mock.go` inside `mocks/` | `todo_repository_mock.go` |

---

## 2. Struct Tags

| Type | `db:` tag | `json:` tag | Reason |
|---|---|---|---|
| Entity | ✅ | ❌ | sqlx struct scanning — never serialized directly to HTTP |
| Value Object | ❌ | ❌ | pure Go struct, compared by value |
| Usecase DTO | ❌ | ✅ | JSON in/out at presentation boundary |
| Infra DTO | ❌ | ✅ | wire-format for external HTTP APIs |

**Never mix `db:` and `json:` tags on the same struct.** Mixing is a sign the struct is crossing layers it shouldn't.

---

## 3. Domain Model Types

Three distinct roles in `internal/domain/model/` — distinguishable by file name, comment, and struct shape:

**Entity** — has identity, persists to DB, carries `db:` tags
```go
// Entity: Todo represents a task persisted in the database.
type Todo struct {
    ID        uint   `db:"id"`
    Title     string `db:"title"`
    ...
}
```

**Value Object** — no identity, no tags, compared by value
```go
// Value Object: TodoFilter describes query criteria for filtering todos.
type TodoFilter struct {
    Done   *bool
    Search *string
}
```

**Domain Rule Constants** — `const` block in `xxx_constant.go`, owned by one entity
```go
// Domain Rule Constants: rules that belong to the Todo entity.
const MaxTitleLength = 255
```

---

## 4. Nullable Fields

Use Go pointers — not `sql.NullString`, `sql.NullInt64`, etc.

```go
// ✅ correct
Description *string
Count       *int

// ❌ avoid
Description sql.NullString
```

Pointer-to-zero-value means the field was explicitly set; `nil` means absent. Consistent across domain model, DTOs, and HTTP binding.

---

## 5. Constants Placement

| Scope | Location | Rule |
|---|---|---|
| Domain rule for one entity | `domain/model/xxx_constant.go` | e.g. `MaxTitleLength` |
| Cross-layer (2+ layers use it) | `internal/constant/` | last resort — default is proximity |
| Infrastructure-only | unexported `const` in the same `.go` file | never export infra constants upward |
| Presentation-only | `internal/presentation/http/constant.go` | e.g. `HeaderRequestID` |

**Default rule: constants live as close as possible to the code that owns them.**

---

## 6. Error Handling

```go
// Wrap errors through AppError before they reach the presentation layer.
if err == sql.ErrNoRows {
    return nil, apperror.NotFound("todo not found")
}

// Log internal details — never send them to the client.
slog.ErrorContext(ctx, "failed to query todos", "error", err)
return nil, apperror.Internal(err)
```

- `apperror.NotFound(msg)` → HTTP 404
- `apperror.BadRequest(msg)` → HTTP 400
- `apperror.Internal(err)` → HTTP 500, original `err` is logged internally only
- Cross-layer error message strings go in `internal/constant/error.go`

---

## 7. Logging

Use `log/slog` (stdlib, Go 1.21+). Never use `fmt.Println` or third-party loggers.

```go
// Structured log with context (request_id, trace_id injected automatically)
slog.InfoContext(ctx, "todo created", "id", todo.ID)
slog.ErrorContext(ctx, "failed to send notification", "error", err)
```

- Set once at startup: `slog.SetDefault(logger)` in `cmd/api/cmd.go`
- Context handler injects `request_id` and `trace_id` from context into every line
- Log at `Error` level only for actual errors; use `Info` or `Debug` for flow events

---

## 8. Context

Always pass `ctx context.Context` as the first parameter for any function that does I/O (DB, HTTP, S3).

```go
// ✅ correct
func (r *todoRepository) GetByID(ctx context.Context, id uint) (*model.Todo, error)

// ❌ avoid — no context means no tracing, no cancellation
func (r *todoRepository) GetByID(id uint) (*model.Todo, error)
```

---

## 9. Constructor Pattern

Constructors return the **interface**, not the concrete struct. This enforces the abstraction at the call site.

```go
// ✅ correct — caller only knows about the interface
func NewTodoUsecase(repo domainRepo.ITodoRepository, notifier domainSvc.INotificationClient) ITodoUsecase {
    return &todoUsecase{repo: repo, notifier: notifier}
}

// ❌ avoid — leaks the concrete type
func NewTodoUsecase(...) *todoUsecase
```

---

## 10. Comments

Write comments only when **why** is non-obvious. Never comment what the code already says.

```go
// ✅ explains a hidden constraint
// Non-fatal: notification failure must not fail the create request.
go func() { _ = u.notifier.Send(ctx, n) }()

// ❌ redundant — the code already says this
// GetByID gets a todo by its ID
func (u *todoUsecase) GetByID(ctx context.Context, id uint) (*dto.TodoOutput, error)
```

File-level comments are the exception — use them to signal the **role** of each domain model file:

```go
// Entity: Todo represents a task persisted in the database.
// Value Object: TodoFilter describes query criteria for filtering todos.
// Domain Rule Constants: rules that belong to the Todo entity.
```

---

## 11. Mocks

- Location: `internal/infrastructure/repository/mocks/`
- Used **only** in usecase unit tests — never in production code
- Hand-written or generated by [mockery](https://github.com/vektra/mockery)
- Pattern: struct with `Fn` fields for easy per-test overrides

```go
type MockTodoRepository struct {
    GetByIDFn func(ctx context.Context, id uint) (*model.Todo, error)
}

func (m *MockTodoRepository) GetByID(ctx context.Context, id uint) (*model.Todo, error) {
    return m.GetByIDFn(ctx, id)
}
```

---

## 12. Dependencies (go.mod)

```
github.com/labstack/echo/v4              — HTTP router
github.com/jmoiron/sqlx                  — DB struct scan with db: tags
github.com/go-sql-driver/mysql           — MySQL driver
github.com/go-playground/validator/v10   — struct validation
github.com/spf13/cobra                   — CLI commands (api, migrate, seed)
github.com/joho/godotenv                 — .env loading
github.com/pressly/goose/v3              — DB migrations
github.com/aws/aws-sdk-go-v2             — S3 client
```

> No ORM (no GORM) — raw SQL + sqlx. Consistent with existing codebase.
