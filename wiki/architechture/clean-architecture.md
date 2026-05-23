# Clean Architecture — Go Base Source

> Analysis based on the real codebase at `pascalia-source` (gRPC + Clean Arch).
> This base source uses **HTTP REST** (Echo) instead of gRPC — easier to use as a starter template.

---

## 1. Dependency Rule

```
Presentation (HTTP handler)
    ↓ calls
Usecase
    ↓ imports
Domain
  model/       — entities + value objects
  repository/  — ITodoRepository        (uses domain model types)
  service/     — INotificationClient    (uses domain model types)
               — IFileStorage           (uses stdlib primitives)
    ↑ all implemented by
Infrastructure
  repository/  — TodoRepository (DB, sqlx)
  httpclient/  — NotificationClient  ──→ maps domain model → infrastructure/dto internally
  s3/          — s3Client              (implements IFileStorage)
```

**One rule: every layer only imports inward — never outward.**

| Layer              | Imports                               | Never imports                        |
| ------------------ | ------------------------------------- | ------------------------------------ |
| `domain`         | stdlib only                           | anything else                        |
| `usecase`        | `domain` only                       | `infrastructure`, `presentation` |
| `infrastructure` | `domain`, `infrastructure/dto`    | `usecase` impl, `presentation`   |
| `presentation`   | `usecase` interfaces, `container` | `infrastructure` directly          |

---

## 2. Interface Placement

Interfaces live in the layer that owns the abstraction or contract.

| Interface               | Lives in               | Reason                                                                             |
| ----------------------- | ---------------------- | ---------------------------------------------------------------------------------- |
| `ITodoRepository`     | `domain/repository/` | usecase consumes it — usecase depends on domain defining this contract            |
| `INotificationClient` | `domain/service/`    | usecase consumes it — usecase depends on domain defining this contract            |
| `IFileStorage`        | `domain/service/`    | usecase consumes it — named after business intent, not the S3 vendor              |
| `ITodoUsecase`        | `usecase/`           | presentation consumes it — presentation depends on usecase defining this contract |

**Why `ITodoUsecase` is NOT in `domain/`:** the usecase interface uses `usecase/dto` types (`CreateTodoInput`, `TodoOutput`). Moving it into domain would force domain to import `usecase/dto`, breaking the rule that domain imports stdlib only.

---

## 3. DTO Placement

| DTO                                         | Lives in                | Used by                                                                                         |
| ------------------------------------------- | ----------------------- | ----------------------------------------------------------------------------------------------- |
| `infrastructure/dto`                      | `infrastructure/dto/` | infrastructure only — never leaks out                                                          |
| `usecase/dto`                             | `usecase/dto/`        | usecase and presentation                                                                        |
| Entity, Value Object, Domain Rule Constants | `domain/model/`       | domain, usecase,**and infrastructure** (repository impl receives/returns `*model.Todo`) |

---

## 4. Directory Structure

```
go-clean-base/
├── main.go
├── go.mod / go.sum
├── .env.example / .gitignore
│
├── cmd/
│   ├── api/cmd.go          # start HTTP server
│   ├── migrate/cmd.go      # goose up/down
│   └── seed/cmd.go         # run seeders (dev/test only)
│
├── config/config.go        # env loading, Config struct
├── container/container.go  # DI container, wires all dependencies
│
├── internal/
│   ├── constant/
│   │   └── common.go       # cross-layer shared constants (DateFormat, Timezone)
│   │
│   ├── domain/
│   │   ├── model/
│   │   │   ├── todo.go              # Entity — db: tags, lifecycle fields
│   │   │   ├── todo_constant.go     # Domain Rule — MaxTitleLength = 255
│   │   │   ├── todo_error.go        # Domain Errors — ErrTodoNotFound
│   │   │   ├── todo_filter.go       # Value Object — query params for ITodoRepository.List
│   │   │   ├── pagination.go        # Value Object — shared across repositories
│   │   │   └── notification.go      # Value Object — used by INotificationClient.Send
│   │   ├── repository/
│   │   │   └── todo_repository.go   # ITodoRepository interface
│   │   ├── repository/
│   │   │   ├── todo_repository.go   # ITodoRepository interface
│   │   │   └── mock/todo_repository_mock.go  # mock lives next to the interface it satisfies
│   │   └── service/
│   │       ├── notification_client.go  # INotificationClient interface
│   │       └── s3_client.go            # IFileStorage interface (vendor-neutral name)
│   │
│   ├── usecase/
│   │   ├── dto/todo_dto.go          # CreateTodoInput, UpdateTodoInput, ListTodoInput, TodoOutput
│   │   ├── todo_usecase.go          # ITodoUsecase interface
│   │   └── todo_usecase_impl.go     # imports domain only
│   │
│   ├── infrastructure/
│   │   ├── database/database.go     # sqlx connection
│   │   ├── repository/
│   │   │   └── todo_repository_impl.go     # implements ITodoRepository
│   │   ├── dto/notification_dto.go  # wire-format JSON — never leaks out of infrastructure
│   │   ├── httpclient/notification_client.go  # implements INotificationClient
│   │   └── s3/s3_client.go         # implements IS3Client
│   │
│   └── presentation/http/
│       ├── server.go
│       ├── constant.go
│       ├── handler/
│       │   ├── todo_handler.go
│       │   └── health_handler.go
│       ├── middleware/
│       │   ├── logger.go
│       │   └── error.go
│       └── validator/todo_validator.go
│
├── pkg/
│   ├── apperror/error.go   # AppError type, error codes
│   ├── logger/logger.go    # slog JSON handler + context handler
│   └── helper/             # pointer, string, time, mask utilities
│
└── db/
    ├── migrations/         # goose Up/Down SQL files
    └── seeds/              # dev/test fixture data
```

---

## 3. Types at Each Layer

| Type         | Location                                   | `db:` tag | `json:` tag |
| ------------ | ------------------------------------------ | ----------- | ------------- |
| Entity       | `domain/model/todo.go`                   | ✅          | ❌            |
| Value Object | `domain/model/todo_filter.go`            | ❌          | ❌            |
| Usecase DTO  | `usecase/dto/todo_dto.go`                | ❌          | ✅            |
| Infra DTO    | `infrastructure/dto/notification_dto.go` | ❌          | ✅            |

**Rule for domain interface signatures:** any type in a `domain/repository/` or `domain/service/` method signature must live in `domain/model/`.

```
ITodoRepository.List(ctx, model.TodoFilter, model.Pagination)  ✅
INotificationClient.Send(ctx, *model.Notification)             ✅
infrastructure/dto.NotificationRequest  — never in domain sig  ✅
usecase/dto.CreateTodoInput             — never in domain sig  ✅
```

---

## 4. Data Flow at Layer Boundaries

```
HTTP JSON body
    ↓ bind + validate (presentation)
usecase/dto.CreateTodoInput
    ↓ map (usecase impl)
domain/model.Todo
    ↓
ITodoRepository.Create(ctx, *model.Todo)   → sqlx INSERT
    ↑ *model.Todo
    ↑ map (usecase impl)
usecase/dto.TodoOutput
    ↑ JSON 201

— — — list with filter — — —
handler binds query params → dto.ListTodoInput
    ↓ map (usecase impl)
model.TodoFilter + model.Pagination
    ↓
ITodoRepository.List(ctx, filter, page)    → sqlx SELECT WHERE ...
    ↑ []*model.Todo → map → []*dto.TodoOutput

— — — notification — — —
usecase builds model.Notification{To, Subject, Body}
    ↓
INotificationClient.Send(ctx, *model.Notification)
    ↓ adapter maps → dto.NotificationRequest → HTTP POST
    ↑ returns messageID string
```

---

## 5. Layer Details

### Domain Layer

**Entity** (`domain/model/todo.go`)

```go
type Todo struct {
    ID          uint      `db:"id"`
    Title       string    `db:"title"`
    Description *string   `db:"description"`
    Done        bool      `db:"done"`
    CreatedAt   time.Time `db:"created_at"`
    UpdatedAt   time.Time `db:"updated_at"`
}
```

**Repository interface** (`domain/repository/todo_repository.go`)

```go
type ITodoRepository interface {
    GetByID(ctx context.Context, id uint) (*model.Todo, error)
    List(ctx context.Context, filter model.TodoFilter, page model.Pagination) ([]*model.Todo, error)
    Create(ctx context.Context, todo *model.Todo) (*model.Todo, error)
    Update(ctx context.Context, todo *model.Todo) (*model.Todo, error)
    Delete(ctx context.Context, id uint) error
}
```

**Service interfaces** (`domain/service/`)

```go
type INotificationClient interface {
    Send(ctx context.Context, n *model.Notification) (string, error)
}

// Named after business intent — not the S3 vendor.
// Swap the infra impl (S3 → GCS) without touching domain or usecase.
type IFileStorage interface {
    Save(ctx context.Context, key string, body io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}
```

### Usecase Layer

**Interface** (`usecase/todo_usecase.go`)

```go
type ITodoUsecase interface {
    GetByID(ctx context.Context, id uint) (*dto.TodoOutput, error)
    List(ctx context.Context, input dto.ListTodoInput) ([]*dto.TodoOutput, error)
    Create(ctx context.Context, input dto.CreateTodoInput) (*dto.TodoOutput, error)
    Update(ctx context.Context, input dto.UpdateTodoInput) (*dto.TodoOutput, error)
    Delete(ctx context.Context, id uint) error
}
```

**Impl** (`usecase/todo_usecase_impl.go`)

- Maps `dto.ListTodoInput` → `model.TodoFilter` + `model.Pagination` before calling repo
- Maps `dto.CreateTodoInput` → `model.Todo` → result → `dto.TodoOutput`
- Sends notification post-create (non-fatal — log error, do not fail the request)

### Infrastructure Layer

**Infra DTO** (`infrastructure/dto/notification_dto.go`) — wire-format only, never leaks out

```go
type NotificationRequest struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}
```

**HTTP client** maps `model.Notification` → `dto.NotificationRequest` internally before HTTP POST.

### Presentation Layer

**Server** (`presentation/http/server.go`)

```go
func NewServer(c *container.Container) *echo.Echo {
    e := echo.New()
    e.Use(middleware.Logger(), middleware.ErrorHandler())

    e.GET("/health", handler.NewHealthHandler().Check)

    v1 := e.Group("/api/v1")
    todo := v1.Group("/todos")
    todo.GET("",        h.List)
    todo.GET("/:id",    h.GetByID)
    todo.POST("",       h.Create)
    todo.PUT("/:id",    h.Update)
    todo.DELETE("/:id", h.Delete)
    return e
}
```

Handler is a thin layer: bind → validate → call usecase → return JSON. No business logic.

### DI Container

```go
func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
    db          := database.NewClient(cfg)
    todoRepo    := infraRepo.NewTodoRepository(db)
    notifier    := httpclient.NewNotificationClient(cfg)
    s3          := s3client.NewS3Client(ctx, cfg)   // ctx needed by aws-sdk-go-v2
    todoUsecase := usecase.NewTodoUsecase(todoRepo, notifier)
    return &Container{...}, nil
}
```

### Error Handling

Domain sentinel errors live in `domain/model/`:

```go
// Domain Errors: sentinel errors for the Todo entity.
var ErrTodoNotFound = errors.New("todo not found")
```

Infrastructure returns domain sentinels — never `apperror` directly:

```go
if err == sql.ErrNoRows {
    return nil, domainModel.ErrTodoNotFound
}
```

The presentation middleware maps domain sentinels → HTTP codes via `errors.Is`:

```go
switch {
case errors.As(err, &appErr):
    // already an AppError — pass through
case errors.Is(err, domainModel.ErrTodoNotFound):
    appErr = apperror.NotFound(err.Error())   // → 404
default:
    appErr = apperror.Internal(err)           // → 500
}
```

`AppError` is the wire type sent to the client:

```go
type AppError struct {
    Code    int
    Message string
    Err     error    // internal only — never sent to client
}
```

Response: `{"error": {"code": 404, "message": "todo not found"}}`

**Why sentinels instead of string constants:**
- `errors.Is` supports wrapping — a sentinel survives `fmt.Errorf("...: %w", err)`
- The domain owns the concept; infrastructure and presentation never need to agree on a string
- Adding a new entity error requires only one line in `domain/model/xxx_error.go` and one `case` in middleware

---

## 6. Verification Checklist

- [ ] `go build ./...` — no errors
- [ ] `go vet ./...` — no warnings
- [ ] `go test ./internal/usecase/...` — passes with mock repo
- [ ] `curl /health` — `200 {"status":"ok"}`
- [ ] `curl -X POST /api/v1/todos` — `201`
- [ ] `curl /api/v1/todos/:id` (missing) — `404` JSON error
- [ ] `curl -X POST /api/v1/todos` (no title) — `400` JSON error
- [ ] `curl "/api/v1/todos?done=false&page=1&limit=10"` — filtered list
- [ ] Log output is JSON with `request_id`
- [ ] `grep -r "infrastructure" internal/domain/` — zero results
- [ ] `grep -r "infrastructure" internal/usecase/` — zero results
- [ ] `docker compose up` — app + mysql start, `/health` reachable
- [ ] `go run main.go migrate` — todos table created
- [ ] `go run main.go seed` — fixture rows inserted
