# T11 — Usecase Layer: Todo Ownership

**Status:** [x] Done

## Depends on
- T10 (ITodoOwnedRepository, ITodoCommentRepository, OwnedTodo model)
- T2 (IUserRepository — needed to resolve email for Share)

## Scope constraint
Usecase imports only `domain`. Must never import `infrastructure` or `presentation`.

## Files to create

### `internal/usecase/dto/todo_owned_dto.go`
```go
package dto

type CreateOwnedTodoInput struct {
    OwnerID     int64   `json:"-"`
    Title       string  `json:"title"       validate:"required,max=255"`
    Description *string `json:"description"`
}

type UpdateOwnedTodoInput struct {
    ID      uint    `json:"-"`
    OwnerID int64   `json:"-"`
    Title   string  `json:"title" validate:"required,max=255"`
    Done    bool    `json:"done"`
}

type OwnedTodoOutput struct {
    ID            uint    `json:"id"`
    Title         string  `json:"title"`
    Description   *string `json:"description"`
    Done          bool    `json:"done"`
    AttachmentURL *string `json:"attachment_url"`
    CreatedAt     string  `json:"created_at"`
    UpdatedAt     string  `json:"updated_at"`
}

type BulkDeleteInput struct {
    IDs []uint `json:"ids" validate:"required,min=1"`
}

type BulkStatusInput struct {
    IDs  []uint `json:"ids"  validate:"required,min=1"`
    Done bool   `json:"done"`
}

type ShareTodoInput struct {
    TodoID      uint   `json:"-"`
    OwnerID     int64  `json:"-"`
    TargetEmail string `json:"email" validate:"required,email"`
}

type AddCommentInput struct {
    TodoID   uint   `json:"-"`
    CallerID int64  `json:"-"`
    Body     string `json:"body" validate:"required,max=2000"`
}

type CommentOutput struct {
    ID        uint   `json:"id"`
    TodoID    uint   `json:"todo_id"`
    UserID    int64  `json:"user_id"`
    Body      string `json:"body"`
    CreatedAt string `json:"created_at"`
}
```

### `internal/usecase/todo_owned_usecase.go`
```go
package usecase

import (
    "context"
    "github.com/yourname/go-clean-base/internal/usecase/dto"
)

type ITodoOwnedUsecase interface {
    ListMine(ctx context.Context, ownerID int64, filter dto.ListTodoInput) ([]*dto.OwnedTodoOutput, error)
    GetMine(ctx context.Context, id uint, ownerID int64) (*dto.OwnedTodoOutput, error)
    CreateMine(ctx context.Context, input dto.CreateOwnedTodoInput) (*dto.OwnedTodoOutput, error)
    UpdateMine(ctx context.Context, input dto.UpdateOwnedTodoInput) (*dto.OwnedTodoOutput, error)
    DeleteMine(ctx context.Context, id uint, ownerID int64) error

    BulkDelete(ctx context.Context, input dto.BulkDeleteInput) error
    BulkSetStatus(ctx context.Context, input dto.BulkStatusInput) error

    ShareTodo(ctx context.Context, input dto.ShareTodoInput) error
    RevokeShare(ctx context.Context, todoID uint, ownerID int64, targetUserID int64) error

    UploadAttachment(ctx context.Context, todoID uint, ownerID int64, fileKey string) (*dto.OwnedTodoOutput, error)
    DeleteAttachment(ctx context.Context, todoID uint, ownerID int64) error

    ListComments(ctx context.Context, todoID uint) ([]*dto.CommentOutput, error)
    AddComment(ctx context.Context, input dto.AddCommentInput) (*dto.CommentOutput, error)
    DeleteComment(ctx context.Context, commentID uint, callerID int64, isAdmin bool) error
}
```

### `internal/usecase/todo_owned_usecase_impl.go`

Constructor:
```go
func NewTodoOwnedUsecase(
    repo        domainrepo.ITodoOwnedRepository,
    userRepo    domainrepo.IUserRepository,
    commentRepo domainrepo.ITodoCommentRepository,
    storage     domainsvc.IFileStorage,
) ITodoOwnedUsecase
```

Key behaviour:
- `GetMine` — return `apperror.Forbidden` if `found.OwnerID != ownerID`
- `UpdateMine` — return `apperror.Forbidden` if not owner
- `DeleteMine` — soft delete via `SoftDeleteOwned`; return `apperror.Forbidden` if not owner
- `BulkDelete` / `BulkSetStatus` — no role check here; route-level `RoleMiddleware(RoleAdmin)` is the guard
- `ShareTodo` — call `IUserRepository.FindByEmail`; return `apperror.NotFound` if email unknown; return `apperror.Forbidden` if `input.OwnerID` does not own the todo
- `DeleteComment` — load comment via `ITodoCommentRepository.FindByID`; return `apperror.Forbidden` if `callerID != comment.UserID && !isAdmin`
- `UploadAttachment` — call `IFileStorage` to get URL, then `ITodoOwnedRepository.UpdateAttachment`

## Verification

```bash
grep -r "infrastructure" internal/usecase/  # must be zero results
go build ./internal/usecase/...
```

## Done when
- [ ] `todo_owned_dto.go` created with all types
- [ ] `todo_owned_usecase.go` interface created
- [ ] `todo_owned_usecase_impl.go` implementation created
- [ ] All methods implement the described behaviour
- [ ] `go build ./internal/usecase/...` passes
- [ ] Zero infrastructure imports in usecase
