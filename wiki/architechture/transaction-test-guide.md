# Transaction Test Guide

Verifies that `WithinTransaction` in the usecase layer commits both writes atomically and rolls back both on failure.

## What is being tested

Every mutating usecase (`Create`, `Update`, `Delete`) wraps two DB writes in a single transaction:

1. The todo operation (INSERT / UPDATE / DELETE on `todos`)
2. An audit log entry (INSERT on `audit_logs`)

Both succeed together or neither is committed.

---

## Setup

```bash
docker compose -f docker/docker-compose.yaml up -d db

# wait for DB to be ready
until docker exec docker-db-1 mysqladmin ping -u root -proot --silent 2>/dev/null; do sleep 1; done

go run main.go migrate
go run main.go api &
```

---

## Test 1 — Commit path (both writes succeed)

```bash
# Clear state
docker exec docker-db-1 mysql -u appuser -papppass appdb \
  -e "DELETE FROM audit_logs; DELETE FROM todos;"

# Create a todo
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Transaction test"}' | jq .
```

**Check both rows were inserted:**

```bash
docker exec docker-db-1 mysql -u appuser -papppass appdb \
  -e "SELECT id, title FROM todos;"

docker exec docker-db-1 mysql -u appuser -papppass appdb \
  -e "SELECT * FROM audit_logs;"
```

**Expected:**
- `todos` contains the new row
- `audit_logs` contains one `create` entry with matching `entity_id`

---

## Test 2 — Rollback path (second write fails)

Simulate an infrastructure failure after the todo insert by dropping `audit_logs`:

```bash
docker exec docker-db-1 mysql -u root -proot appdb \
  -e "DROP TABLE audit_logs;"

# Attempt create — todo INSERT succeeds, audit INSERT fails
curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Should be rolled back"}' | jq .
```

**Expected response:**
```json
{
  "error": {
    "code": 500,
    "message": "internal server error"
  }
}
```

**Check no orphan todo row was committed:**

```bash
docker exec docker-db-1 mysql -u appuser -papppass appdb \
  -e "SELECT id, title FROM todos;"
```

**Expected:** `"Should be rolled back"` is absent — the todo INSERT was rolled back together with the failed audit INSERT.

> Note: MySQL auto-increment advances even on rollback, so the next successful insert will skip an ID. This is expected behaviour, not a bug.

---

## Test 3 — Recovery (normal operation resumes after restore)

```bash
# Restore audit_logs table
docker exec docker-db-1 mysql -u root -proot appdb -e "
CREATE TABLE audit_logs (
    id         INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    entity     VARCHAR(50)  NOT NULL,
    entity_id  INT UNSIGNED NOT NULL,
    action     VARCHAR(20)  NOT NULL,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_audit_logs_entity_id (entity, entity_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;"

curl -s -X POST http://localhost:8080/api/v1/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"After restore"}' | jq .
```

**Expected:** todo and audit_log both inserted normally.

---

## Teardown

```bash
kill $(pgrep -f "go run main.go api")
docker compose -f docker/docker-compose.yaml down
```

---

## Summary

| Test | Scenario | Expected |
|---|---|---|
| Commit path | both writes succeed | todo + audit_log both present |
| Rollback path | second write fails | neither row committed |
| Recovery | table restored | normal commit resumes |
