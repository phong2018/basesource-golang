// lib/helpers.js — shared helpers for all k6 scripts
import http from 'k6/http';
import { check } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:18080';

export const JSON_HEADERS = { 'Content-Type': 'application/json' };

// randomTitle generates a unique todo title to avoid duplicate-key issues.
export function randomTitle(prefix) {
  return `${prefix} ${Date.now()} ${Math.random().toString(36).slice(2, 7)}`;
}

// createTodo POSTs a new todo and checks HTTP 201.
// Returns the parsed response body (or null on failure).
export function createTodo(title) {
  const res = http.post(
    `${BASE_URL}/api/v1/todos`,
    JSON.stringify({ title: title || randomTitle('perf') }),
    { headers: JSON_HEADERS },
  );
  check(res, { 'create: HTTP 201': (r) => r.status === 201 });
  try {
    return JSON.parse(res.body);
  } catch (_) {
    return null;
  }
}

// listTodos GETs /api/v1/todos and checks HTTP 200.
export function listTodos() {
  const res = http.get(`${BASE_URL}/api/v1/todos`);
  check(res, { 'list: HTTP 200': (r) => r.status === 200 });
  return res;
}

// getTodo GETs /api/v1/todos/:id and checks HTTP 200.
export function getTodo(id) {
  const res = http.get(`${BASE_URL}/api/v1/todos/${id}`);
  check(res, { 'get: HTTP 200': (r) => r.status === 200 });
  return res;
}

// updateTodo PUTs /api/v1/todos/:id and checks HTTP 200.
// title is required by the API — always pass it.
export function updateTodo(id, title, done) {
  const res = http.put(
    `${BASE_URL}/api/v1/todos/${id}`,
    JSON.stringify({ title: title || randomTitle('updated'), done: done !== undefined ? done : true }),
    { headers: JSON_HEADERS },
  );
  check(res, { 'update: HTTP 200': (r) => r.status === 200 });
  return res;
}

// deleteTodo DELETEs /api/v1/todos/:id and checks HTTP 204.
export function deleteTodo(id) {
  const res = http.del(`${BASE_URL}/api/v1/todos/${id}`);
  check(res, { 'delete: HTTP 204': (r) => r.status === 204 });
  return res;
}

// healthCheck GETs /health and checks HTTP 200.
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  check(res, { 'health: HTTP 200': (r) => r.status === 200 });
  return res;
}
