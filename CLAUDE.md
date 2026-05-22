# CLAUDE.md — Go Clean Architecture Base Source

## Project Overview

Go REST API template using Clean Architecture (Hexagonal). Todo CRUD is the working example covering the full flow end-to-end.

- **HTTP framework:** Echo v4
- **Database:** MySQL + sqlx (raw SQL, no ORM)
- **Migrations:** goose v3
- **CLI:** Cobra (`api`, `migrate`, `seed` commands)
- **Object storage:** aws-sdk-go-v2 S3
- **Logging:** `log/slog` (stdlib)

Full docs in [`wiki/architechture/`](wiki/architechture/).

---

## Architecture — Dependency Rule

```
Presentation → Usecase → Domain ← Infrastructure
```

| Layer | May import | Must never import |
|---|---|---|
| `domain` | stdlib only | anything else |
| `usecase` | `domain` only | `infrastructure`, `presentation` |
| `infrastructure` | `domain`, `infrastructure/dto` | `usecase` impl, `presentation` |
| `presentation` | `usecase` interfaces, `container` | `infrastructure` directly |

**Verify the rule at any time:**
```bash
grep -r "infrastructure" internal/domain/     # must be zero results
grep -r "infrastructure" internal/usecase/    # must be zero results
```

---

## Key Conventions

### Naming
- Interface: prefix `I` — `ITodoUsecase`, `ITodoRepository`
- Constructor: `NewXxx(deps...) IXxx` — returns interface, not struct
- Impl file: `xxx_impl.go`
- File names: `snake_case.go`

### Struct tags — never mix on the same struct
| Type | `db:` | `json:` |
|---|---|---|
| Entity (`domain/model/`) | ✅ | ❌ |
| Value Object (`domain/model/`) | ❌ | ❌ |
| Usecase DTO (`usecase/dto/`) | ❌ | ✅ |
| Infra DTO (`infrastructure/dto/`) | ❌ | ✅ |

### Domain model file roles (signal via comment + struct shape)
- `// Entity:` — has ID, `db:` tags, maps 1:1 to DB row
- `// Value Object:` — no ID, no tags, compared by value
- `// Domain Rule Constants:` — `const` block, owned by one entity

### Nullable fields
Use `*string`, `*int` — never `sql.NullString`.

### Constants placement
- Domain rule for one entity → `domain/model/xxx_constant.go`
- Cross-layer (2+ layers) → `internal/constant/`
- Infra-only → unexported `const` in the same file
- Presentation-only → `internal/presentation/http/constant.go`

### Error handling
```go
// Wrap before reaching presentation — never expose raw errors
return nil, apperror.NotFound("todo not found")
return nil, apperror.Internal(err)  // err logged internally, not sent to client
```

### Logging
```go
slog.InfoContext(ctx, "todo created", "id", todo.ID)
slog.ErrorContext(ctx, "failed to query", "error", err)
```
Always `slog.XxxContext(ctx, ...)` — context carries `request_id` and `trace_id`.

### Context
Always `ctx context.Context` as the first parameter on any function doing I/O.

---

## Running Locally

```bash
cp .env.example .env
docker compose -f docker/docker-compose.yaml up -d db
go run main.go migrate
go run main.go seed      # optional fixtures
go run main.go api       # starts on :8080
```

```bash
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy milk"}'
```

---

## Adding a New Feature

1. **Domain** — add entity/value object in `internal/domain/model/`, add interface in `internal/domain/repository/` or `internal/domain/service/`
2. **Usecase** — add DTOs in `internal/usecase/dto/`, add interface + impl
3. **Infrastructure** — implement the repository/service adapter
4. **Presentation** — add handler, register route in `server.go`
5. **Container** — wire in `container/container.go`
6. **Migration** — add SQL file in `db/migrations/` with goose `-- +goose Up` / `-- +goose Down`

---

## Verification Checklist

```bash
go build ./...                        # must pass
go vet ./...                          # must pass
go test ./internal/usecase/...        # must pass
grep -r "infrastructure" internal/domain/   # zero results
grep -r "infrastructure" internal/usecase/  # zero results
```
