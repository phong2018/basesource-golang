# T12 — Infrastructure Layer: Todo Ownership Repositories

**Status:** [ ] Not started

## Depends on
- T9 (DB tables must exist)
- T10 (ITodoOwnedRepository, ITodoCommentRepository interfaces)

## Files to create

### `internal/infrastructure/repository/todo_owned_repository_impl.go`

Implement `ITodoOwnedRepository` using `sqlx` raw SQL.

Key queries:

```sql
-- ListByOwner (soft-delete aware)
SELECT id, owner_id, title, description, done, deleted_at, attachment_url, created_at, updated_at
FROM todos WHERE owner_id = ? AND deleted_at IS NULL
[optional: AND done = ?] [optional: AND title LIKE ?]
ORDER BY created_at DESC LIMIT ? OFFSET ?

-- FindOwned (ownership check in SQL, not application layer)
SELECT id, owner_id, title, description, done, deleted_at, attachment_url, created_at, updated_at
FROM todos WHERE id = ? AND owner_id = ? AND deleted_at IS NULL LIMIT 1

-- CreateOwned
INSERT INTO todos (owner_id, title, description, done, created_at, updated_at)
VALUES (:owner_id, :title, :description, false, NOW(), NOW())

-- UpdateOwned
UPDATE todos SET title=:title, description=:description, done=:done, updated_at=NOW()
WHERE id=:id AND owner_id=:owner_id

-- SoftDeleteOwned
UPDATE todos SET deleted_at=NOW() WHERE id=? AND owner_id=?

-- BulkSoftDelete
UPDATE todos SET deleted_at=NOW() WHERE id IN (?)  -- use sqlx.In

-- BulkSetStatus
UPDATE todos SET done=?, updated_at=NOW() WHERE id IN (?)

-- Share
INSERT IGNORE INTO todo_shares (todo_id, user_id, created_at) VALUES (?, ?, NOW())

-- RevokeShare
DELETE FROM todo_shares WHERE todo_id=? AND user_id=?

-- UpdateAttachment
UPDATE todos SET attachment_url=?, updated_at=NOW() WHERE id=? AND owner_id=?
```

Constructor:
```go
func NewTodoOwnedRepository(db *sqlx.DB) domain.ITodoOwnedRepository
```

### `internal/infrastructure/repository/todo_comment_repository_impl.go`

Implement `ITodoCommentRepository` using `sqlx` raw SQL.

Key queries:
```sql
-- List
SELECT id, todo_id, user_id, body, created_at FROM todo_comments
WHERE todo_id=? ORDER BY created_at ASC

-- Create
INSERT INTO todo_comments (todo_id, user_id, body, created_at) VALUES (:todo_id, :user_id, :body, NOW())

-- FindByID
SELECT id, todo_id, user_id, body, created_at FROM todo_comments WHERE id=? LIMIT 1

-- Delete
DELETE FROM todo_comments WHERE id=?
```

Constructor:
```go
func NewTodoCommentRepository(db *sqlx.DB) domain.ITodoCommentRepository
```

## Verification

```bash
go build ./internal/infrastructure/...

# Dependency rule
grep -r "usecase"      internal/infrastructure/  # must be zero results
grep -r "presentation" internal/infrastructure/  # must be zero results
```

## Done when
- [ ] `todo_owned_repository_impl.go` created, implements all 10 methods
- [ ] `todo_comment_repository_impl.go` created, implements all 4 methods
- [ ] `go build ./internal/infrastructure/...` passes
- [ ] Zero usecase/presentation imports in infrastructure
