# T10 — Domain Layer: Todo Ownership Models & Interfaces

**Status:** [x] Done

## Depends on
- T2 (existing domain conventions established)

## Scope constraint
- `internal/domain/model/todo.go` is **not modified**
- Domain must never import `infrastructure` or `usecase`

## Files to create

### `internal/domain/model/todo_owned.go`
```go
package model

import "time"

// Value Object: extends Todo with ownership fields for the owned-todo surface.
// Used by ITodoOwnedRepository queries — does not replace the Todo entity.
type OwnedTodo struct {
    ID            uint       `db:"id"`
    OwnerID       *int64     `db:"owner_id"`
    Title         string     `db:"title"`
    Description   *string    `db:"description"`
    Done          bool       `db:"done"`
    DeletedAt     *time.Time `db:"deleted_at"`
    AttachmentURL *string    `db:"attachment_url"`
    CreatedAt     time.Time  `db:"created_at"`
    UpdatedAt     time.Time  `db:"updated_at"`
}
```

### `internal/domain/model/todo_comment.go`
```go
package model

import "time"

// Entity: comment on a todo
type TodoComment struct {
    ID        uint      `db:"id"`
    TodoID    uint      `db:"todo_id"`
    UserID    int64     `db:"user_id"`
    Body      string    `db:"body"`
    CreatedAt time.Time `db:"created_at"`
}
```

### `internal/domain/model/todo_share.go`
```go
package model

import "time"

// Entity: share relationship between a todo and a user
type TodoShare struct {
    TodoID    uint      `db:"todo_id"`
    UserID    int64     `db:"user_id"`
    CreatedAt time.Time `db:"created_at"`
}
```

### `internal/domain/repository/todo_owned_repository.go`
```go
package repository

import (
    "context"
    "github.com/yourname/go-clean-base/internal/domain/model"
)

// ITodoOwnedRepository handles ownership-aware todo queries.
// Does not extend ITodoRepository — entirely separate surface.
type ITodoOwnedRepository interface {
    ListByOwner(ctx context.Context, ownerID int64, filter model.TodoFilter) ([]*model.OwnedTodo, error)
    FindOwned(ctx context.Context, id uint, ownerID int64) (*model.OwnedTodo, error)
    CreateOwned(ctx context.Context, todo *model.OwnedTodo) error
    UpdateOwned(ctx context.Context, todo *model.OwnedTodo) error
    SoftDeleteOwned(ctx context.Context, id uint, ownerID int64) error
    BulkSoftDelete(ctx context.Context, ids []uint) error
    BulkSetStatus(ctx context.Context, ids []uint, done bool) error
    Share(ctx context.Context, todoID uint, targetUserID int64) error
    RevokeShare(ctx context.Context, todoID uint, targetUserID int64) error
    UpdateAttachment(ctx context.Context, id uint, ownerID int64, url *string) error
}
```

### `internal/domain/repository/todo_comment_repository.go`
```go
package repository

import (
    "context"
    "github.com/yourname/go-clean-base/internal/domain/model"
)

type ITodoCommentRepository interface {
    List(ctx context.Context, todoID uint) ([]*model.TodoComment, error)
    Create(ctx context.Context, comment *model.TodoComment) error
    FindByID(ctx context.Context, id uint) (*model.TodoComment, error)
    Delete(ctx context.Context, id uint) error
}
```

## Verification

```bash
grep -r "infrastructure" internal/domain/  # must be zero results
grep -r "usecase"        internal/domain/  # must be zero results
go build ./internal/domain/...
```

## Done when
- [ ] All 5 files created
- [ ] `go build ./internal/domain/...` passes
- [ ] Zero infrastructure/usecase imports in domain
