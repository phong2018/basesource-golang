#!/usr/bin/env bash
# =============================================================================
# zap_scan.sh — Automated OWASP ZAP Security Scan
# =============================================================================
#
# PURPOSE
#   Runs a full OWASP ZAP security scan against the locally running API and
#   writes three reports into the zap/ directory.
#
# PREREQUISITES (must be done before calling this script)
#   1. Infrastructure running:
#        docker compose -f docker/docker-compose.yaml up -d db rabbitmq kafka
#   2. Migrations applied:
#        go run main.go migrate
#   3. API server running:
#        nohup go run main.go api > /tmp/api.log 2>&1 &
#        curl http://localhost:8080/health   # wait for 200
#   4. Docker Desktop running (ZAP runs inside a container)
#   5. JWT_SECRET set in .env (any non-empty string)
#
# USAGE
#   chmod +x scripts/zap_scan.sh
#   ./scripts/zap_scan.sh
#
# OUTPUTS
#   zap/zap-baseline-report.html      — passive scan (no attacks, ~2 min)
#   zap/zap-authenticated-report.html — active authenticated scan with JWT (~8 min)
#   zap/manual-checks-report.md       — 9 access control checks with PASS/FAIL
#
# EXIT CODES
#   0 — all scans completed (manual checks may still have failures, check report)
#   1 — preflight failed (API not running, Docker not running, token error)
#
# macOS NOTE
#   Docker Desktop on macOS cannot use --network host. This script uses
#   host.docker.internal to reach the host from inside the ZAP container.
#   On Linux, replace host.docker.internal with localhost and add --network host.
# =============================================================================
set -euo pipefail

API=http://localhost:8080
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR="$REPO_ROOT/zap"
mkdir -p "$REPORT_DIR"

pass() { echo "[PASS] $*"; }
fail() { echo "[FAIL] $*"; }
info() { echo "[INFO] $*"; }
header() {
  echo ""
  echo "=================================================="
  echo "  $*"
  echo "=================================================="
}

# ── Preflight ─────────────────────────────────────────────────────────────────
header "Preflight checks"

curl -sf "$API/health" > /dev/null || { echo "ERROR: API not reachable at $API — start the server first"; exit 1; }
pass "API reachable"

docker info > /dev/null 2>&1 || { echo "ERROR: Docker not running"; exit 1; }
pass "Docker running"

# ── Seed users ────────────────────────────────────────────────────────────────
header "Seeding test users"

REGISTER_ADMIN=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}')
[ "$REGISTER_ADMIN" = "201" ] && pass "admin registered" || info "admin already exists (HTTP $REGISTER_ADMIN)"

REGISTER_USER=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"Test1234!"}')
[ "$REGISTER_USER" = "201" ] && pass "user registered" || info "user already exists (HTTP $REGISTER_USER)"

# Promote admin via DB
docker compose -f "$REPO_ROOT/docker/docker-compose.yaml" exec -T db \
  mysql -u root -proot appdb \
  -e "UPDATE users SET role='admin' WHERE email='admin@test.com';" 2>/dev/null
pass "admin role set"

# Capture tokens
ADMIN_TOKEN=$(curl -s -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234!"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

USER_TOKEN=$(curl -s -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"Test1234!"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

[ -n "$ADMIN_TOKEN" ] && pass "admin token acquired" || { echo "ERROR: failed to get admin token"; exit 1; }
[ -n "$USER_TOKEN" ]  && pass "user token acquired"  || { echo "ERROR: failed to get user token"; exit 1; }

# Create a todo owned by user for cross-ownership test
TODO_ID=$(curl -s -X POST "$API/api/v1/my/todos" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"ZAP scan test todo"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
[ -n "$TODO_ID" ] && pass "test todo created id=$TODO_ID" || info "could not create test todo (non-fatal)"

# ── Scan 1: Baseline passive scan ────────────────────────────────────────────
header "Scan 1 — ZAP Baseline passive scan (unauthenticated)"
info "Passive only — no attack payloads. Checks missing headers, info disclosure, misconfigurations."
info "Report: $REPORT_DIR/zap-baseline-report.html"

docker run --rm \
  -v "$REPORT_DIR:/zap/wrk" \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-baseline.py \
  -t "http://host.docker.internal:8080/api/v1/todos" \
  -r zap-baseline-report.html \
  -I 2>&1 | tail -20

pass "Baseline scan complete — report: zap/zap-baseline-report.html"

# ── Scan 2: Authenticated API scan (OpenAPI-driven) ──────────────────────────
header "Scan 2 — ZAP API scan (authenticated, OpenAPI-driven)"
info "Active scan — attacks every endpoint listed in zap/openapi.yaml with auth token."
info "Uses zap-api-scan.py so ZAP probes /my/todos, /admin/todos, /auth/*, etc. directly."
info "Report: $REPORT_DIR/zap-authenticated-report.html"

# zap-api-scan.py reads every path from openapi.yaml and attacks each one directly.
# This ensures /my/todos, /admin/todos, /todos/:id/comments etc. are all scanned —
# unlike zap-full-scan.py which relies on spidering HTML links and misses JSON-only APIs.
# The replacer rule injects Authorization: Bearer <token> on every request so protected
# routes return 200/204 rather than 401.
docker run --rm \
  -v "$REPORT_DIR:/zap/wrk" \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-api-scan.py \
  -t /zap/wrk/openapi.yaml \
  -f openapi \
  -z "-config replacer.full_list(0).description=jwt -config replacer.full_list(0).enabled=true -config replacer.full_list(0).matchtype=REQ_HEADER -config replacer.full_list(0).matchstr=Authorization -config replacer.full_list(0).replacement=Bearer\\ $ADMIN_TOKEN" \
  -r zap-authenticated-report.html \
  -I 2>&1 | tail -30

pass "API scan complete — report: zap/zap-authenticated-report.html"

# ── Scan 3: Manual broken access control checks ───────────────────────────────
header "Scan 3 — Manual access control checks"

MANUAL_REPORT="$REPORT_DIR/manual-checks-report.md"
PASS_COUNT=0
FAIL_COUNT=0

check() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" = "$expected" ]; then
    echo "| $desc | $expected | $actual | PASS |" >> "$MANUAL_REPORT"
    pass "$desc → $actual"
    PASS_COUNT=$((PASS_COUNT+1))
  else
    echo "| $desc | $expected | $actual | **FAIL** |" >> "$MANUAL_REPORT"
    fail "$desc → expected $expected, got $actual"
    FAIL_COUNT=$((FAIL_COUNT+1))
  fi
}

# Init report file
cat > "$MANUAL_REPORT" <<'MD'
# ZAP Manual Access Control Checks

| Scenario | Expected | Actual | Result |
|---|---|---|---|
MD

# 1 — No token → 401
SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/my/todos")
check "No token on GET /my/todos" "401" "$SC"

# 2 — Expired/tampered JWT → 401
SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/my/todos" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.dGFtcGVyZWQ.badsig")
check "Tampered JWT → 401" "401" "$SC"

# 3 — User accesses another user's todo (cross-ownership)
if [ -n "$TODO_ID" ]; then
  SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/my/todos/$TODO_ID" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
  check "Admin token accesses user-owned todo (non-owner)" "403" "$SC"
else
  echo "| Cross-ownership check | 403 | SKIP (no todo) | SKIP |" >> "$MANUAL_REPORT"
  info "Cross-ownership check skipped — no test todo"
fi

# 4 — User tries admin bulk-delete → 403
SC=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/admin/todos/bulk-delete" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1,2,3]}')
check "User hits admin bulk-delete → 403" "403" "$SC"

# 5 — User tries admin bulk-status → 403
SC=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$API/api/v1/admin/todos/bulk-status" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[1],"done":true}')
check "User hits admin bulk-status → 403" "403" "$SC"

# 6 — Admin can bulk-delete (non-existent IDs → 200 not 403)
SC=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/admin/todos/bulk-delete" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[999999]}')
{ [ "$SC" = "200" ] || [ "$SC" = "204" ] || [ "$SC" = "404" ]; } && RESULT_6="PASS" || RESULT_6="FAIL"
echo "| Admin bulk-delete with valid token | 200 or 204 or 404 | $SC | $RESULT_6 |" >> "$MANUAL_REPORT"
[ "$RESULT_6" = "PASS" ] && { pass "Admin bulk-delete → $SC (not 403)"; PASS_COUNT=$((PASS_COUNT+1)); } || { fail "Admin bulk-delete → unexpected $SC"; FAIL_COUNT=$((FAIL_COUNT+1)); }

# 7 — Unauthenticated access to /me → 401
SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/me")
check "No token on GET /me → 401" "401" "$SC"

# 8 — Public GET /todos still works without token
SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/todos")
check "Public GET /todos without token → 200" "200" "$SC"

# 9 — SQL injection in login body (should not crash)
SC=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com\" OR \"1\"=\"1","password":"anything"}')
[ "$SC" != "500" ] && RESULT_SQL="PASS" || RESULT_SQL="FAIL"
echo "| SQL injection in login body | not 500 | $SC | $RESULT_SQL |" >> "$MANUAL_REPORT"
[ "$RESULT_SQL" = "PASS" ] && { pass "SQL injection in login → $SC (no crash)"; PASS_COUNT=$((PASS_COUNT+1)); } || { fail "SQL injection in login → 500 (crash!)"; FAIL_COUNT=$((FAIL_COUNT+1)); }

# ── Authenticated success cases (confirm /my and /admin are reachable with token) ────────────

# 10 — User token → GET /my/todos → 200
SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/my/todos" \
  -H "Authorization: Bearer $USER_TOKEN")
check "User token → GET /my/todos → 200" "200" "$SC"

# 11 — User token → POST /my/todos → 201
SC=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/my/todos" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"ZAP auth verify"}')
check "User token → POST /my/todos → 201" "201" "$SC"

# 12 — Admin token → GET /my/todos (admin sees own list) → 200
SC=$(curl -s -o /dev/null -w "%{http_code}" "$API/api/v1/my/todos" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
check "Admin token → GET /my/todos → 200" "200" "$SC"

# 13 — Admin token → POST /admin/todos/bulk-delete → 204
SC=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/api/v1/admin/todos/bulk-delete" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[999998,999997]}')
check "Admin token → POST /admin/todos/bulk-delete → 204" "204" "$SC"

# 14 — Admin token → PATCH /admin/todos/bulk-status → 204
SC=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$API/api/v1/admin/todos/bulk-status" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[999998,999997],"done":true}')
check "Admin token → PATCH /admin/todos/bulk-status → 204" "204" "$SC"

# Append summary
cat >> "$MANUAL_REPORT" <<MD

## Summary

- **Total checks:** $((PASS_COUNT + FAIL_COUNT))
- **Passed:** $PASS_COUNT
- **Failed:** $FAIL_COUNT
- **Date:** $(date '+%Y-%m-%d %H:%M:%S')
MD

# ── Final report ──────────────────────────────────────────────────────────────
header "Scan complete"
echo ""
echo "Reports written to:"
echo "  $REPORT_DIR/zap-baseline-report.html"
echo "  $REPORT_DIR/zap-authenticated-report.html"
echo "  $REPORT_DIR/manual-checks-report.md"
echo ""
echo "Manual checks: $PASS_COUNT passed, $FAIL_COUNT failed"
echo ""
[ "$FAIL_COUNT" -eq 0 ] && echo "All checks PASSED ✅" || echo "Some checks FAILED ❌ — review $REPORT_DIR/manual-checks-report.md"
