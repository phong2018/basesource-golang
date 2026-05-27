# T7 — Config: JWT Environment Variables

**Status:** [x] Done

## Depends on
- None (standalone)

## What to do

### 1. Add to `.env.example`

```
JWT_SECRET=change-me-in-production
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_DAYS=7
```

### 2. Add to `.env` (local development copy)

```
JWT_SECRET=local-dev-secret-do-not-use-in-prod
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_DAYS=7
```

### 3. Add to config struct (wherever `cfg` is loaded in the project)

```go
JWTSecret           string
JWTAccessTTLMinutes int
JWTRefreshTTLDays   int
```

Read with defaults:
```go
JWTSecret:           getEnv("JWT_SECRET", ""),
JWTAccessTTLMinutes: getEnvInt("JWT_ACCESS_TTL_MINUTES", 15),
JWTRefreshTTLDays:   getEnvInt("JWT_REFRESH_TTL_DAYS", 7),
```

> If `JWT_SECRET` is empty at startup, the server should fail fast with a clear error rather than starting with an insecure empty secret.

## Verification

```bash
# Confirm keys are present in .env.example
grep "JWT_" .env.example
# Expected: all 3 lines appear

# Server refuses to start with no secret
JWT_SECRET="" go run main.go api
# Expected: startup error mentioning JWT_SECRET

# Server starts with secret set
go run main.go api
curl http://localhost:8080/health
# Expected: 200
```

## Done when
- [ ] `.env.example` has all 3 JWT keys
- [ ] `.env` (local) has all 3 JWT keys
- [ ] Config struct reads all 3 values
- [ ] Server fails fast when `JWT_SECRET` is empty
