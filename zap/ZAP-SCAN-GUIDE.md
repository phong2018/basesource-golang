# OWASP ZAP Security Scan — Complete Guide

This guide explains every step we took to run a full OWASP ZAP security scan on this project,
what each step does, and how to re-run it yourself from scratch.

---

## Why We Did This

OWASP ZAP (Zed Attack Proxy) is an open-source security scanner. It probes your running API
for vulnerabilities like:

- SQL injection
- Cross-site scripting (XSS)
- Missing security headers
- Broken access control
- Information disclosure

We added JWT-based authentication and role-based authorization (T1–T13) specifically to give
ZAP more surface area to test — authenticated routes, ownership checks, and admin-only endpoints.

---

## Prerequisites

| Tool | Purpose |
|---|---|
| Docker | Runs the ZAP scanner container |
| Go | Builds and runs the API |
| MySQL (via Docker Compose) | Database |
| RabbitMQ (via Docker Compose) | Required by the API to start |
| `jq` or `python3` | Parses JSON responses in shell |

---

## Step 1 — Start Infrastructure

```bash
# Start MySQL + RabbitMQ
docker compose -f docker/docker-compose.yaml up -d db rabbitmq kafka
```

Wait until they are ready:

```bash
# Check MySQL
docker compose -f docker/docker-compose.yaml exec db mysqladmin ping -u root -proot --silent

# Check RabbitMQ
docker compose -f docker/docker-compose.yaml exec rabbitmq rabbitmq-diagnostics ping --silent
```

**Why:** The API refuses to start without a database connection and a RabbitMQ connection.
Both must be healthy before the API can serve requests.

---

## Step 2 — Set JWT Secret in .env

```bash
# .env must contain these (add if missing)
JWT_SECRET=any-non-empty-secret-value
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_DAYS=7
```

**Why:** The JWT middleware reads `JWT_SECRET` at startup. Without it, `GenerateAccessToken`
and `ValidateAccessToken` will fail for every request, making all auth tests useless.

---

## Step 3 — Run Database Migrations

```bash
go run main.go migrate
```

**Why:** Migrations create the `users`, `refresh_tokens`, `todo_shares`, and `todo_comments`
tables that the auth and ownership features depend on. ZAP cannot test login, register, or
ownership endpoints if those tables don't exist.

---

## Step 4 — Start the API Server

```bash
nohup go run main.go api > /tmp/api.log 2>&1 &

# Wait until it responds
curl http://localhost:8080/health
# Expected: 200
```

**Why:** ZAP scans a live running server. It sends real HTTP requests and observes real HTTP
responses, so the API must be up and reachable on port 8080.

---

## Step 5 — Seed Test Users

```bash
# Register admin user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}'

# Register regular user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"Test1234!"}'

# Promote admin (register always creates role=user)
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u root -proot appdb \
  -e "UPDATE users SET role='admin' WHERE email='admin@test.com';"
```

**Why:** ZAP's authenticated scan needs real credentials to log in. The manual access control
checks need two distinct roles (admin vs user) to verify that role enforcement works.
The `register` endpoint always sets `role=user`; the DB update is the only way to promote to admin.

---

## Step 6 — Capture JWT Tokens

```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

USER_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"Test1234!"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

echo "ADMIN_TOKEN=$ADMIN_TOKEN"
echo "USER_TOKEN=$USER_TOKEN"
```

**Why:** The manual access control checks (Step 9) use these tokens directly in `Authorization`
headers to verify that:
- Protected routes reject requests without a token (401)
- User-role tokens are rejected on admin-only routes (403)
- Admin tokens can access admin routes

---

## Step 7 — Create Output Directory

```bash
mkdir -p zap
```

All reports are written into the `zap/` folder at the project root.

---

## Step 8 — ZAP Scan 1: Baseline Passive Scan

```bash
docker run --rm \
  -v "$(pwd)/zap:/zap/wrk:rw" \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-baseline.py \
  -t http://host.docker.internal:8080/api/v1/todos \
  -r zap-baseline-report.html \
  -d -I
```

**Report:** `zap/zap-baseline-report.html`

### What this scan does

The baseline scan is a **passive** scan — it does NOT send attack payloads. It only:

1. Spiders the target URL to discover linked URLs
2. Passively analyses all responses it receives
3. Checks for common misconfigurations:
   - Missing security headers (`X-Content-Type-Options`, `X-Frame-Options`, etc.)
   - Information disclosure in error messages
   - Cookies without `HttpOnly`/`Secure` flags
   - Content served without `Cache-Control`

### Why `host.docker.internal` instead of `localhost`

On **macOS**, Docker Desktop runs inside a VM. `--network host` has no effect. The container
cannot reach `localhost:8080` on your Mac. `host.docker.internal` is a special DNS name Docker
Desktop provides that resolves to the host machine's IP from inside a container.

On **Linux**, use `--network host` and `localhost` instead:
```bash
docker run --rm --network host \
  -v "$(pwd)/zap:/zap/wrk:rw" \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-baseline.py -t http://localhost:8080/api/v1/todos -r zap-baseline-report.html -I
```

### Why `-I` flag

`-I` means "ignore failures" — the scan exits with code 0 even if there are warnings. Without
it, any warning causes a non-zero exit and the shell script stops. We use `-I` so the scan
always completes and generates a full report even if issues are found.

### Why target `/api/v1/todos` not `/`

The root path `/` had no handler and returned 500. ZAP's spider uses the start URL as an entry
point; a 500 immediately causes it to abort with "spider error". Starting at a valid endpoint
gives ZAP a useful page to spider from. (We later fixed the root 500 — see "Fixes Applied".)

---

## Step 9 — ZAP Scan 2: Authenticated API Scan (OpenAPI-driven)

### Why `zap-api-scan.py` instead of `zap-full-scan.py`

`zap-full-scan.py` discovers URLs by **spidering** — it starts at the target URL, reads the
response, and follows any HTML links it finds. This works for websites. It does **not** work
for REST APIs because a JSON response like `[{"id":1,"title":"Buy milk"}]` contains no links.

Result: ZAP only finds 9 guessed URLs and **never reaches `/my/todos` or `/admin/todos`**.

```
zap-full-scan.py              zap-api-scan.py
─────────────────             ──────────────────────────────
Starts at /api/v1/todos  →    Reads zap/openapi.yaml
Spiders for HTML links   →    Finds ALL 16 paths directly
Finds ~9 URLs            →    Attacks /my/todos ✅
Misses /my/todos ❌             Attacks /admin/todos ✅
Misses /admin/todos ❌          Attacks /auth/login ✅
```

`zap-api-scan.py` reads an **OpenAPI spec** (`zap/openapi.yaml`) that lists every endpoint
explicitly. ZAP attacks each path directly — no spidering, no missed routes.

### Step 9a — The OpenAPI spec (`zap/openapi.yaml`)

This file tells ZAP every route that exists. It is committed at `zap/openapi.yaml`.
Add new routes here whenever you add new endpoints.

```yaml
openapi: "3.0.0"
servers:
  - url: http://host.docker.internal:8080
paths:
  /api/v1/my/todos:       # ZAP will attack GET and POST
    get: {}
    post: {}
  /api/v1/my/todos/{id}:  # ZAP will attack GET, PUT, DELETE with id=1,2,...
    get:
      parameters: [{name: id, in: path, required: true, schema: {type: integer}}]
    ...
  /api/v1/admin/todos/bulk-delete:
    post: {}
```

### Step 9b — How auth injection works (ZAP replacer rule)

`zap-api-scan.py` does **not** support an auth-context YAML file — that YAML is for ZAP's
Automation Framework, not the CLI scanner. The correct approach is to obtain a JWT token on
the host first (via `curl /auth/login`), then pass it to ZAP via the **replacer** rule using
`-z`. ZAP's replacer middleware injects `Authorization: Bearer <token>` on every outgoing
request before it reaches the API.

```
Host: curl /auth/login → gets JWT
         │
         ▼
ZAP replacer rule: REQ_HEADER Authorization = "Bearer <jwt>"
         │
         ▼
Every request ZAP sends → carries the JWT automatically
```

The `zap_scan.sh` script captures `$ADMIN_TOKEN` before starting Scan 2 and passes it via:

```bash
-z "-config replacer.full_list(0).description=jwt \
    -config replacer.full_list(0).enabled=true \
    -config replacer.full_list(0).matchtype=REQ_HEADER \
    -config replacer.full_list(0).matchstr=Authorization \
    -config replacer.full_list(0).replacement=Bearer <token>"
```

This approach is more reliable than ZAP's built-in session management for JWT APIs because:
- No ZAP login dialog or automation YAML needed
- Token is obtained and validated on the host before scanning starts
- Works with `zap-api-scan.py` which has no `-H` header flag

### Step 9c — Run the scan

```bash
# Obtain token first
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

docker run --rm \
  -v "$(pwd)/zap:/zap/wrk:rw" \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-api-scan.py \
  -t /zap/wrk/openapi.yaml \
  -f openapi \
  -z "-config replacer.full_list(0).description=jwt -config replacer.full_list(0).enabled=true -config replacer.full_list(0).matchtype=REQ_HEADER -config replacer.full_list(0).matchstr=Authorization -config replacer.full_list(0).replacement=Bearer $ADMIN_TOKEN" \
  -r zap-authenticated-report.html \
  -I
```

| Flag | Meaning |
|---|---|
| `-t /zap/wrk/openapi.yaml` | OpenAPI spec path **inside the container** (mounted via `-v`) |
| `-f openapi` | Spec format: OpenAPI (vs. SOAP, GraphQL) |
| `-z "-config replacer..."` | Injects JWT header on every request ZAP sends |
| `-r zap-authenticated-report.html` | Output report (written to `zap/` on host) |
| `-I` | Exit 0 even if alerts found — so the script always completes |

**Report:** `zap/zap-authenticated-report.html`

### How to verify ZAP successfully scanned `/admin` and `/my` routes

ZAP does not produce a simple "login success" message. You verify auth by looking at the
**response codes** ZAP received on protected routes.

**The rule:**
```
401 = token was missing or rejected  (auth FAILED)
400 / 403 / 404 / 204 = server processed the request past auth  (auth WORKED)
```

ZAP is an attack scanner — it sends fuzz/attack payloads, not normal requests. So protected
routes will rarely return `200`. Instead:
- `GET /my/todos` with a valid token but a fuzz payload in params → `400` (bad request)
- `GET /my/todos/10` as admin (not owner of todo #10) → `403` (ownership check)
- `POST /admin/todos/bulk-delete` with a garbage body from ZAP → `400` (bad request)

All of these confirm the JWT was accepted. A `401` on any of these would mean auth failed.

**Quick check after a scan run:**

```bash
python3 -c "
import re
with open('zap/zap-authenticated-report.html') as f:
    content = f.read()
pattern = r'(\/api\/v1\/(?:my|admin)[^\s<\"]+).*?Evidence.*?<td[^>]*>(.*?)<\/td>'
matches = re.findall(pattern, content, re.DOTALL)
seen = {}
for url, ev in matches:
    if url not in seen:
        seen[url] = re.sub(r'<[^>]+>', '', ev).strip()
auth_failed = [u for u,c in seen.items() if c == '401']
print('Auth FAILED routes (401):', len(auth_failed))
for u in auth_failed: print(' ', u)
if not auth_failed:
    print('  None — JWT injected correctly ✅')
print()
print('Sample /my and /admin responses:')
for url, code in sorted(seen.items()):
    if ('/my/' in url or '/admin/' in url) and code:
        print(f'  {code:5s}  {url}')
"
```

Expected output when auth is working:
```
Auth FAILED routes (401): 0
  None — JWT injected correctly ✅

Sample /my and /admin responses:
   400  /api/v1/admin/todos/bulk-delete
   400  /api/v1/admin/todos/bulk-status
   403  /api/v1/my/todos/10
   400  /api/v1/my/todos/10/attachment
   ...
```

If you see `401` on `/my/` or `/admin/` routes, the JWT was not injected — check that
`$ADMIN_TOKEN` was captured successfully in the script output (`[PASS] admin token acquired`).

### What attacks this scan runs

| Attack | What ZAP tries |
|---|---|
| SQL Injection | `' OR '1'='1`, time-based blind injection on all parameters |
| XSS (Reflected) | `<script>alert(1)</script>` in URL params and POST bodies |
| XSS (Persistent) | Stored payloads via POST to `/my/todos`, `/todos/:id/comments` |
| Path Traversal | `../../etc/passwd` in URL path segments |
| CRLF Injection | Newline characters in header values |
| Command Injection | Shell metacharacters (`; ls`, `| id`) in inputs |
| Log4Shell | `${jndi:ldap://...}` in all string fields |
| Buffer Overflow | Oversized strings in all parameters |
| CORS Misconfiguration | `Origin: evil.com` header injection |
| Auth bypass | Missing/invalid tokens on all protected routes |

---

## Step 10 — Manual Access Control Checks

These are curl-based checks that ZAP's automated scanner cannot verify by itself — specifically
**broken access control** (OWASP A01) scenarios:

```bash
# 1. No token → must get 401
curl -s -o/dev/null -w"%{http_code}" http://localhost:8080/api/v1/my/todos
# Expected: 401

# 2. Tampered JWT → must get 401
curl -s -o/dev/null -w"%{http_code}" http://localhost:8080/api/v1/my/todos \
  -H "Authorization: Bearer bad.token.sig"
# Expected: 401

# 3. Non-owner accessing another user's todo → must get 403
curl -s -o/dev/null -w"%{http_code}" http://localhost:8080/api/v1/my/todos/5 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
# Expected: 403  (todo id=5 was created by user@test.com, not admin)

# 4. User hitting admin-only endpoint → must get 403
curl -s -o/dev/null -w"%{http_code}" \
  -X POST http://localhost:8080/api/v1/admin/todos/bulk-delete \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" -d '{"ids":[1]}'
# Expected: 403

# 5. Admin CAN use admin endpoint → must NOT get 403
curl -s -o/dev/null -w"%{http_code}" \
  -X POST http://localhost:8080/api/v1/admin/todos/bulk-delete \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" -d '{"ids":[999999]}'
# Expected: 200 or 204 (not 403)

# 6. SQL injection in login body → must not crash (no 500)
curl -s -o/dev/null -w"%{http_code}" \
  -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"'"'"' OR '"'"'1'"'"'='"'"'1","password":"x"}'
# Expected: 401 (not 500)

# 7. Public endpoint still works without any token
curl -s -o/dev/null -w"%{http_code}" http://localhost:8080/api/v1/todos
# Expected: 200
```

**Why ZAP can't do these automatically:** ZAP doesn't understand your business logic. It doesn't
know that todo ID 5 belongs to `user@test.com` and that `admin@test.com` shouldn't see it via
`/my/todos`. These IDOR (Insecure Direct Object Reference) and ownership checks require custom
scripts or manual verification.

---

## Step 11 — Automated Script (`scripts/zap_scan.sh`)

### What it is

`scripts/zap_scan.sh` is a single shell script that automates **all of Steps 5–10** above.
Instead of copying and running each curl and docker command manually, you run one script and
it handles everything end-to-end.

### What it does — in order

```
zap_scan.sh
│
├── Preflight
│   ├── Checks API is reachable at localhost:8080/health  → exits if not
│   └── Checks Docker is running                         → exits if not
│
├── Seed users
│   ├── POST /auth/register  admin@test.com   (skips if already exists)
│   ├── POST /auth/register  user@test.com    (skips if already exists)
│   ├── UPDATE users SET role='admin'         (via docker compose exec db mysql)
│   ├── POST /auth/login → captures ADMIN_TOKEN
│   └── POST /auth/login → captures USER_TOKEN
│
├── Create test todo
│   └── POST /my/todos with USER_TOKEN → captures TODO_ID for cross-ownership check
│
├── Scan 1: ZAP Baseline passive scan
│   ├── Runs ghcr.io/zaproxy/zaproxy:stable  zap-baseline.py
│   └── Writes zap/zap-baseline-report.html
│
├── Scan 2: ZAP API scan (OpenAPI-driven, authenticated)
│   ├── Reads  zap/openapi.yaml            (all 16 routes — no spidering needed)
│   ├── Injects JWT via replacer rule      (-z "-config replacer..." Bearer $ADMIN_TOKEN)
│   ├── Runs ghcr.io/zaproxy/zaproxy:stable  zap-api-scan.py  -f openapi
│   └── Writes zap/zap-authenticated-report.html
│
└── Scan 3: Manual access control checks (9 curl checks)
    ├── No token → 401
    ├── Tampered JWT → 401
    ├── Non-owner accessing user todo → 403
    ├── User hitting admin bulk-delete → 403
    ├── User hitting admin bulk-status → 403
    ├── Admin hitting admin bulk-delete → 200/204 (not 403)
    ├── No token on /me → 401
    ├── Public GET /todos without token → 200
    ├── SQL injection in login body → not 500
    └── Writes zap/manual-checks-report.md with PASS/FAIL per check
```

### What it does NOT do

The script does **not** start the server. You must start the API yourself before calling it.
This is intentional — in CI, the server is started in a separate step, and the script just
runs against whatever is already running.

### How to run it

```bash
# One-time setup
chmod +x scripts/zap_scan.sh

# Run (API must already be running)
./scripts/zap_scan.sh
```

### Files in `zap/`

| File | Status | Purpose |
|---|---|---|
| `zap/scripts/zap_scan.sh` | committed | Runs all three scans end-to-end |
| `zap/openapi.yaml` | committed | OpenAPI spec — tells ZAP every route to attack |
| `zap/ZAP-SCAN-GUIDE.md` | committed | This document |
| `zap/zap-baseline-report.html` | generated | Passive scan results (open in browser) |
| `zap/zap-authenticated-report.html` | generated | Active authenticated scan results (open in browser) |
| `zap/manual-checks-report.md` | generated | PASS/FAIL table for the 9 manual checks |

Generated files are overwritten on each run. They are committed so the latest scan results are visible without re-running.

### What a successful run looks like

```
==================================================
  Preflight checks
==================================================
[PASS] API reachable
[PASS] Docker running

==================================================
  Seeding test users
==================================================
[INFO] admin already exists (HTTP 409)
[INFO] user already exists (HTTP 409)
[PASS] admin role set
[PASS] admin token acquired
[PASS] user token acquired
[PASS] test todo created id=7

==================================================
  Scan 1 — ZAP Baseline passive scan (unauthenticated)
==================================================
Total of 7 URLs
FAIL-NEW: 0   WARN-NEW: 1   PASS: 66
[PASS] Baseline scan complete — report: zap/zap-baseline-report.html

==================================================
  Scan 2 — ZAP API scan (authenticated, OpenAPI-driven)
==================================================
Number of Imported URLs: 23
Total of 80 URLs
FAIL-NEW: 0   WARN-NEW: 0   PASS: 119
[PASS] API scan complete — report: zap/zap-authenticated-report.html

==================================================
  Scan 3 — Manual access control checks
==================================================
[PASS] #1  No token → GET /my/todos → 401
[PASS] #2  Tampered JWT → protected route → 401
[PASS] #3  Non-owner (admin) accesses user-owned todo → 403
[PASS] #4  User token → admin bulk-delete → 403
[PASS] #5  User token → admin bulk-status → 403
[PASS] #6  Admin token → admin bulk-delete (valid) → 204
[PASS] #7  No token → GET /me → 401
[PASS] #8  Public GET /todos (no token) → 200
[PASS] #9  SQL injection in login body → 401
[PASS] #10 Empty Bearer token → 401
[PASS] #11 Duplicate email registration → 409
[PASS] #12 Valid user token → GET /my/todos → 200
[PASS] #13 Valid admin token → GET /me → 200
[PASS] #14 Empty title → no 500 → 400

==================================================
  Scan complete
==================================================
Manual checks: 14 passed, 0 failed
All checks PASSED ✅
```

### What to do if a check fails

| Symptom | Likely cause | Fix |
|---|---|---|
| `ERROR: API not reachable` | Server not started | `go run main.go api &` |
| `ERROR: Docker not running` | Docker Desktop not open | Start Docker Desktop |
| `ERROR: failed to get admin token` | DB not seeded or wrong password | Re-run with a clean DB or check `.env` |
| `[FAIL] ... expected 401 got 200` | JWT middleware not wired | Check `server.go` route group uses `JWTMiddleware` |
| `[FAIL] ... expected 403 got 200` | Role middleware missing | Check route group uses `RoleMiddleware(RoleAdmin)` |
| `[FAIL] SQL injection → 500` | Raw string interpolation in query | Use parameterized queries (`?` placeholders with sqlx) |
| ZAP baseline shows High findings | Active vulnerability | Read the HTML report, fix the flagged endpoint |

---

## Fixes Applied During Scanning

ZAP's first run found 4 warnings. We fixed them:

### Fix 1 — 500 on unknown routes (`/`, `/robots.txt`, `/sitemap.xml`)

**Problem:** Echo returned 500 for any route it couldn't match.

**Root cause:** Two issues combined:
1. Echo had no fallback handler for unmatched routes.
2. The `ErrorHandler` middleware converted all `*echo.HTTPError` (including Echo's built-in 404)
   into a 500 `AppError` because the `switch` had no case for `*echo.HTTPError`.

**Fix in `server.go`:**
```go
e.RouteNotFound("/*", func(c echo.Context) error {
    return echo.NewHTTPError(404, "not found")
})
```

**Fix in `middleware/error.go`:**
```go
case errors.As(err, &echoErr):
    msg, _ := echoErr.Message.(string)
    appErr = apperror.New(echoErr.Code, msg, nil)
```

**Result:** Unknown paths now return `{"error":{"code":404,"message":"not found"}}`.

### Fix 2 — Missing `Cross-Origin-Resource-Policy` header

**Problem:** ZAP flagged `[90004] Cross-Origin-Resource-Policy Header Missing`.

**Why this matters:** Without this header, other origins can embed your API responses in
`<img>`, `<script>`, or `fetch()` calls, potentially leaking data via side channels.

**Fix in `server.go`** (inline middleware):
```go
c.Response().Header().Set("Cross-Origin-Resource-Policy", "same-origin")
```

### Fix 3 — Missing `Cache-Control` header

**Problem:** ZAP flagged `[10049] Storable and Cacheable Content`.

**Why this matters:** If API responses are cached by a proxy or browser, sensitive data
(user lists, todo content) could be served to the wrong user from cache.

**Fix in `server.go`:**
```go
c.Response().Header().Set("Cache-Control", "no-store")
```

### Fix 4 — `SELECT *` on `todos` table after adding new columns

**Problem (not a security issue — a compatibility bug):** After migration 006 added `owner_id`,
`deleted_at`, `attachment_url` columns to the `todos` table, the old `todoRepository` used
`SELECT *`. sqlx failed with `missing destination name owner_id in *[]*model.Todo` because
the `Todo` struct had no `db:"owner_id"` tag.

**Fix in `todo_repository_impl.go`:**
```go
// Before
"SELECT * FROM todos WHERE id = ?"

// After
"SELECT id, title, description, done, created_at, updated_at FROM todos WHERE id = ?"
```

**Why this was the right fix:** The old `Todo` struct intentionally does NOT know about
`owner_id` — that field belongs to `OwnedTodo`. Selecting only the columns the struct knows
about keeps the two models cleanly separated.

---

## Final Scan Results

**Last run:** 2026-05-27

| | Baseline | API Scan (Authenticated, OpenAPI) |
|---|---|---|
| URLs scanned | 7 | **80** |
| High | 0 ✅ | 0 ✅ |
| Medium | 0 ✅ | 0 ✅ |
| Warnings | 1 (accepted) | **0** ✅ |
| Manual checks | — | 9/9 ✅ |
| Auth working | — | ✅ (401→403/400 on protected routes) |

> **Auth verification:** Protected routes `/my/todos` and `/admin/todos/bulk-delete` returned
> `403` (not `401`) during the API scan, confirming ZAP injected the JWT correctly. A `403`
> means the server recognized the token and enforced ownership rules; `401` would mean the
> token was missing.

> **Key improvement:** Switched from `zap-full-scan.py` (spider-based, 9 URLs) to
> `zap-api-scan.py` with `zap/openapi.yaml` (OpenAPI-driven, **80 URLs**) — every route
> including `/my/todos`, `/admin/todos`, `/auth/*`, and `/todos/:id/comments` is now scanned.

### Accepted Remaining Warning

**`Non-Storable Content [10049]`** — ZAP flags that 404 error responses carry `Cache-Control: no-store`.

This is expected and correct: error responses should not be cached. No action needed.

**`HTTP Only Site [10106]`** (baseline only) — ZAP flags that the site runs on HTTP, not HTTPS.

This is expected and accepted: the development environment runs HTTP on `localhost:8080`.
In production, all traffic is HTTPS-terminated at the ingress/load balancer before reaching
the Go service. The Go service itself never needs to handle TLS.

---

## How to Re-Run the Scan

```bash
# 1. Make sure the app is running
docker compose -f docker/docker-compose.yaml up -d db rabbitmq kafka
go run main.go migrate
nohup go run main.go api > /tmp/api.log 2>&1 &
curl http://localhost:8080/health   # wait for 200

# 2. Run the automated scan script
./scripts/zap_scan.sh

# 3. Open reports
open zap/zap-baseline-report.html
open zap/zap-authenticated-report.html
cat zap/manual-checks-report.md
```

---

## Understanding the Report HTML Files

Open `zap/zap-baseline-report.html` in a browser. You will see:

| Section | Meaning |
|---|---|
| **Risk: High** (red) | Must fix before production |
| **Risk: Medium** (orange) | Fix or document accepted risk |
| **Risk: Low** (yellow) | Review; fix if cheap |
| **Informational** (blue) | Awareness only; no action required |
| **False Positive** | ZAP flagged something that is not actually a vulnerability |

Each alert shows:
- The affected URL
- The HTTP request ZAP sent
- The HTTP response that triggered the alert
- A description of the vulnerability class
- A recommended fix

---

## Security Headers Summary (Added by This Project)

All responses from this API now include:

| Header | Value | Purpose |
|---|---|---|
| `X-XSS-Protection` | `1; mode=block` | Enables browser XSS filter |
| `X-Content-Type-Options` | `nosniff` | Prevents MIME sniffing |
| `X-Frame-Options` | `SAMEORIGIN` | Prevents clickjacking |
| `Content-Security-Policy` | `default-src 'self'` | Restricts resource origins |
| `Strict-Transport-Security` | `max-age=31536000` | Forces HTTPS for 1 year |
| `Cross-Origin-Resource-Policy` | `same-origin` | Prevents cross-origin embedding |
| `Cache-Control` | `no-store` | Prevents caching of API responses |
