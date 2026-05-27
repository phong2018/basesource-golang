# T9 — DB Migration: Todo Ownership, Shares & Comments

**Status:** [ ] Not started

## Depends on
- T1 (users table must exist — foreign key reference)

## Scope constraint
The existing `todos` table structure is preserved. Only new columns and new tables are added.

## What to create

File: `db/migrations/20260527000006_create_todo_ownership.sql`

```sql
-- +goose Up
ALTER TABLE todos
    ADD COLUMN owner_id        BIGINT UNSIGNED NULL,
    ADD COLUMN deleted_at      DATETIME NULL,
    ADD COLUMN attachment_url  VARCHAR(500) NULL,
    ADD CONSTRAINT fk_todos_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL;

CREATE TABLE todo_shares (
    todo_id    BIGINT UNSIGNED NOT NULL,
    user_id    BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (todo_id, user_id),
    FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE todo_comments (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    todo_id    BIGINT UNSIGNED NOT NULL,
    user_id    BIGINT UNSIGNED NOT NULL,
    body       TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS todo_comments;
DROP TABLE IF EXISTS todo_shares;
ALTER TABLE todos
    DROP FOREIGN KEY fk_todos_owner,
    DROP COLUMN owner_id,
    DROP COLUMN deleted_at,
    DROP COLUMN attachment_url;
```

## Verification

```bash
# Run migration (T1 must already be applied)
go run main.go migrate

# Confirm new columns on todos
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u root -proot basesource \
  -e "DESCRIBE todos;"
# Expected: owner_id, deleted_at, attachment_url columns present

# Confirm new tables
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u root -proot basesource \
  -e "SHOW TABLES;"
# Expected: todo_shares and todo_comments in list

# Confirm rollback
go run main.go migrate down
# Expected: columns and tables removed cleanly

go run main.go migrate
```

## Done when
- [ ] Migration file created at correct path
- [ ] `go run main.go migrate` succeeds
- [ ] `todos` table has `owner_id`, `deleted_at`, `attachment_url` columns
- [ ] `todo_shares` and `todo_comments` tables exist
- [ ] `go run main.go migrate down` removes additions cleanly
