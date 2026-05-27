# T15 — OWASP ZAP Security Scan

**Status:** [ ] Not started

## Depends on
- T1–T14 complete and server running
- Docker installed

## Prerequisites

```bash
# Start the app
cp .env.example .env   # set JWT_SECRET to a non-empty value
docker compose -f docker/docker-compose.yaml up -d db
go run main.go migrate
go run main.go api &

curl http://localhost:8080/health
# Expected: 200
```

## Step 1 — Seed test users

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}'

curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"Test1234!"}'

# Promote admin (register always creates role=user)
docker compose -f docker/docker-compose.yaml exec db \
  mysql -u root -proot basesource \
  -e "UPDATE users SET role='admin' WHERE email='admin@test.com';"

# Capture tokens
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}' | jq -r '.access_token')

USER_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"Test1234!"}' | jq -r '.access_token')

echo "ADMIN_TOKEN=$ADMIN_TOKEN"
echo "USER_TOKEN=$USER_TOKEN"
```

## Step 2 — Create output folder

```bash
mkdir -p zap
```

## Scan 1 — Baseline passive scan (unauthenticated)

Catches missing headers, information disclosure, obvious misconfigurations.

```bash
docker run --rm \
  --network host \
  -v $(pwd)/zap:/zap/wrk \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-baseline.py \
  -t http://localhost:8080 \
  -r /zap/wrk/zap-baseline-report.html
```

Report: `zap/zap-baseline-report.html`

## Scan 2 — Authenticated full active scan

Create `zap/auth-context.yaml`:
```yaml
env:
  contexts:
    - name: "API with JWT"
      urls:
        - "http://localhost:8080"
      authentication:
        method: "http"
        parameters:
          loginUrl: "http://localhost:8080/api/v1/auth/login"
          loginRequestData: '{"email":"admin@test.com","password":"Test1234!"}'
          loginPageValidation: "response.status == 200"
      sessionManagement:
        method: "httpAuthHeader"
        parameters:
          headerName: "Authorization"
          headerValue: "Bearer {%json:access_token%}"
      users:
        - name: "admin"
          credentials:
            username: "admin@test.com"
            password: "Test1234!"
        - name: "regular-user"
          credentials:
            username: "user@test.com"
            password: "Test1234!"
```

```bash
docker run --rm \
  --network host \
  -v $(pwd)/zap:/zap/wrk \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-full-scan.py \
  -t http://localhost:8080 \
  -z "-configfile /zap/wrk/auth-context.yaml" \
  -r /zap/wrk/zap-authenticated-report.html
```

Report: `zap/zap-authenticated-report.html`

## Scan 3 — Manual broken access control checks

```bash
# 401 — no token
curl -i http://localhost:8080/api/v1/my/todos
# Expected: 401

# 403 — user accesses another user's todo
curl -i http://localhost:8080/api/v1/my/todos/999 \
  -H "Authorization: Bearer $USER_TOKEN"
# Expected: 403

# 403 — user tries admin bulk-delete
curl -i -X POST http://localhost:8080/api/v1/admin/todos/bulk-delete \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1,2,3]}'
# Expected: 403

# 401 — tampered JWT
curl -i http://localhost:8080/api/v1/my/todos \
  -H "Authorization: Bearer eyJtYW1wZXJlZA.payload.badsig"
# Expected: 401

# 403 — non-owner deletes another user's comment
curl -i -X DELETE http://localhost:8080/api/v1/todos/1/comments/5 \
  -H "Authorization: Bearer $USER_TOKEN"
# Expected: 403

# 200 — admin can bulk-delete
curl -i -X POST http://localhost:8080/api/v1/admin/todos/bulk-delete \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[999]}'
# Expected: 200 or 404, NOT 403
```

## Security test matrix

| Scenario | Endpoint | Expected | Method |
|---|---|---|---|
| No token | `GET /my/todos` | 401 | Manual |
| Expired token | any protected | 401 | Manual |
| Tampered JWT | any protected | 401 | Manual |
| `user` accesses another user's todo | `GET /my/todos/:id` | 403 | Manual |
| `user` hits admin bulk-delete | `POST /admin/todos/bulk-delete` | 403 | Manual |
| `user` hits admin bulk-status | `PATCH /admin/todos/bulk-status` | 403 | Manual |
| Non-owner deletes comment | `DELETE /todos/:id/comments/:cid` | 403 | Manual |
| SQL injection in login body | `POST /auth/login` | 401 (no crash) | ZAP active |
| SQL injection in `?search=` param | `GET /my/todos?search=` | 200 (sanitised) | ZAP active |
| SQL injection in comment body | `POST /todos/:id/comments` | 400 or 201 (no crash) | ZAP active |
| Oversized payload in comment body | `POST /todos/:id/comments` | 400 | ZAP active |
| IDOR — access todo by guessing ID | `GET /my/todos/:id` | 403 | ZAP active |
| Bulk-delete with huge ID array | `POST /admin/todos/bulk-delete` | 400 | ZAP active |
| File upload with executable MIME | `POST /my/todos/:id/attachment` | 400 | ZAP active |
| Brute-force login | `POST /auth/login` | 429 (rate limit) | ZAP active |
| Missing security headers | all routes | Medium alert | ZAP baseline |
| Sensitive data in error body | all routes | Low/Info | ZAP baseline |
| CSRF on state-changing routes | POST/PUT/DELETE | Check | ZAP baseline |
| Token returned in URL | any | High alert | ZAP baseline |

## Interpreting results

| Risk level | Action |
|---|---|
| High | Fix before merge |
| Medium | Fix or document accepted risk |
| Low / Informational | Review; fix if cheap |

## Verification / Done when

- [ ] `zap/zap-baseline-report.html` generated — zero High findings
- [ ] `zap/zap-authenticated-report.html` generated — zero High findings
- [ ] All 7 manual Scan 3 checks return expected status codes
- [ ] All 19 security matrix scenarios documented as pass/fail
- [ ] Any Medium findings reviewed and either fixed or marked accepted risk with justification
