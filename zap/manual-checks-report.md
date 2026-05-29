# ZAP Manual Access Control Checks

| Scenario | Expected | Actual | Result |
|---|---|---|---|
| No token on GET /my/todos | 401 | 401 | PASS |
| Tampered JWT → 401 | 401 | 401 | PASS |
| Admin token accesses user-owned todo (non-owner) | 404 | 404 | PASS |
| User hits admin bulk-delete → 403 | 403 | 403 | PASS |
| User hits admin bulk-status → 403 | 403 | 403 | PASS |
| Admin bulk-delete with valid token | 200 or 204 or 404 | 204 | PASS |
| No token on GET /me → 401 | 401 | 401 | PASS |
| Public GET /todos without token → 200 | 200 | 200 | PASS |
| SQL injection in login body | not 500 | 401 | PASS |
| User token → GET /my/todos → 200 | 200 | 200 | PASS |
| User token → POST /my/todos → 201 | 201 | 201 | PASS |
| Admin token → GET /my/todos → 200 | 200 | 200 | PASS |
| Admin token → POST /admin/todos/bulk-delete → 204 | 204 | 204 | PASS |
| Admin token → PATCH /admin/todos/bulk-status → 204 | 204 | 204 | PASS |

## Summary

- **Total checks:** 14
- **Passed:** 14
- **Failed:** 0
- **Date:** 2026-05-29 12:47:09
