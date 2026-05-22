# Go Clean Architecture Base Source

A production-ready Go project template following Clean Architecture (Hexagonal Architecture) with a Todo CRUD example.

## Architecture

```
┌─────────────────────────────────────────────┐
│              Presentation Layer             │
│         (HTTP handlers, middleware)         │
├─────────────────────────────────────────────┤
│               Usecase Layer                 │
│         (business logic, DTOs)              │
├─────────────────────────────────────────────┤
│            Infrastructure Layer             │
│    (DB, HTTP clients, S3, repositories)     │
├─────────────────────────────────────────────┤
│               Domain Layer                  │
│     (entities, interfaces, value objects)   │
└─────────────────────────────────────────────┘
```

**Dependency rule:** domain ← usecase ← infrastructure ← presentation. Inner layers never import outer layers.

| Layer | Imports | Never imports |
|---|---|---|
| `domain` | stdlib only | anything else |
| `usecase` | `domain` only | `infrastructure`, `presentation` |
| `infrastructure` | `domain` | `usecase` impl, `presentation` |
| `presentation` | `usecase` interfaces, `container` | `infrastructure` directly |

## Directory Structure

```
.
├── cmd/
│   ├── api/            # start HTTP server
│   ├── migrate/        # run DB migrations
│   └── seed/           # load seed data
├── config/             # config struct + env loading
├── container/          # dependency injection wiring
├── db/
│   ├── migrations/     # goose SQL migration files
│   └── seeds/          # dev/test fixture data
├── docker/
│   ├── Dockerfile
│   ├── docker-compose.yaml
│   └── db/my.cnf
├── internal/
│   ├── constant/       # cross-layer constants and error codes
│   ├── domain/
│   │   ├── model/      # entities (db: tags) + value objects (no tags)
│   │   ├── repository/ # repository interfaces (ports)
│   │   └── service/    # external service interfaces (ports)
│   ├── infrastructure/
│   │   ├── database/   # sqlx connection
│   │   ├── dto/        # wire-format structs for external APIs
│   │   ├── httpclient/ # HTTP client implementations
│   │   ├── repository/ # DB repository implementations (adapters)
│   │   │   └── mocks/  # hand-written mocks for unit tests
│   │   └── s3/         # S3 client implementation
│   ├── presentation/
│   │   └── http/
│   │       ├── handler/    # route handlers
│   │       ├── middleware/ # logger, error handler
│   │       └── validator/  # request validation
│   └── usecase/
│       ├── dto/        # input/output structs (usecase ↔ presentation)
│       ├── todo_usecase.go       # interface
│       └── todo_usecase_impl.go  # implementation
├── pkg/
│   ├── apperror/   # AppError type with code + message
│   ├── helper/     # generics, time, masking utilities
│   └── logger/     # slog JSON logger with context handler
└── main.go
```

## Getting Started

### Prerequisites

- Go 1.23+
- Docker + Docker Compose
- [goose](https://github.com/pressly/goose) (installed automatically via `go mod`)

### Local Setup

```bash
# 1. Copy env file
cp .env.example .env

# 2. Start MySQL
docker compose -f docker/docker-compose.yaml up -d db

# 3. Run migrations
go run main.go migrate

# 4. (Optional) Load seed data
go run main.go seed

# 5. Start the server
go run main.go api
```

The server starts on `http://localhost:8080`.

### Docker (full stack)

```bash
docker compose -f docker/docker-compose.yaml up --build
```

## API Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check |
| POST | `/api/v1/todos` | Create a todo |
| GET | `/api/v1/todos` | List todos (with filter + pagination) |
| GET | `/api/v1/todos/:id` | Get a todo by ID |
| PUT | `/api/v1/todos/:id` | Update a todo |
| DELETE | `/api/v1/todos/:id` | Delete a todo |

### Query Parameters for List

| Param | Type | Description |
|---|---|---|
| `done` | bool | Filter by completion status |
| `search` | string | Search in title |
| `page` | int | Page number (default: 1) |
| `limit` | int | Items per page (default: 20) |

### Example Requests

```bash
# Create
curl -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Buy groceries","description":"Milk, eggs, bread"}'

# List with filter
curl "http://localhost:8080/api/v1/todos?done=false&search=buy&page=1&limit=10"
```

## Adding a New Feature

1. **Domain** — add entity in `internal/domain/model/`, add interface in `internal/domain/repository/` or `internal/domain/service/`
2. **Usecase** — add input/output DTOs in `internal/usecase/dto/`, add interface + implementation
3. **Infrastructure** — implement the repository/service adapter
4. **Presentation** — add handler in `internal/presentation/http/handler/`, register route in `server.go`
5. **Container** — wire everything in `container/container.go`
6. **Migration** — add a new SQL file in `db/migrations/` with goose annotations

## Tech Stack

| Concern | Library |
|---|---|
| HTTP framework | [Echo v4](https://echo.labstack.com/) |
| Database | MySQL + [sqlx](https://github.com/jmoiron/sqlx) |
| Migrations | [goose v3](https://github.com/pressly/goose) |
| Object storage | [aws-sdk-go-v2 S3](https://github.com/aws/aws-sdk-go-v2) |
| Logging | `log/slog` (stdlib) |
| CLI | [Cobra](https://github.com/spf13/cobra) |
| Config | [godotenv](https://github.com/joho/godotenv) |

## Environment Variables

See [.env.example](.env.example) for all available configuration options.
