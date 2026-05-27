# T1 — DB Migration: Auth Tables

**Status:** [ ] Not started

## Scope constraint
Additive only. No existing migration files are modified.

## What to create

File: `db/migrations/20260527000005_create_users_table.sql`

```sql
-- +goose Up
CREATE TABLE users (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email      VARCHAR(255) NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,
    role       ENUM('admin','user') NOT NULL DEFAULT 'user',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE refresh_tokens (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT UNSIGNED NOT NULL,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    revoked    TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
```

## Verification

```bash
# Run migration
go run main.go migrate

# Confirm tables exist
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u root -proot basesource -e "SHOW TABLES;"
# Expected: refresh_tokens and users appear in the list

# Confirm schema
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u root -proot basesource -e "DESCRIBE users; DESCRIBE refresh_tokens;"
# Expected: all columns present with correct types

# Confirm rollback works
go run main.go migrate down
go run main.go migrate
```

## Done when
- [ ] Migration file exists at the correct path
- [ ] `go run main.go migrate` runs without error
- [ ] `users` and `refresh_tokens` tables exist in DB with correct columns
- [ ] `go run main.go migrate down` removes both tables cleanly
