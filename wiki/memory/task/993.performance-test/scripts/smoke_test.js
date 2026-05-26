// smoke_test.js — sanity check: 1 VU, 30s, all endpoints once per iteration
//   Purpose: confirm the environment is healthy before running load tests.
//   Expected: 100% success, P95 < 100ms.
//   Run: k6 run wiki/memory/task/993.performance-test/scripts/smoke_test.js
import { sleep } from 'k6';
import { createTodo, listTodos, getTodo, updateTodo, deleteTodo, healthCheck } from './lib/helpers.js';

export const options = {
  vus: 1,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<100'],
    http_req_failed:   ['rate<0.01'],
  },
};

export default function () {
  // 1. health check
  healthCheck();

  // 2. create a todo
  const todo = createTodo('smoke test');
  if (!todo || !todo.id) { sleep(1); return; }

  // 3. get it back
  getTodo(todo.id);

  // 4. list all
  listTodos();

  // 5. update it — title is required by the API
  updateTodo(todo.id, 'smoke test updated', true);

  // 6. delete it — clean up so DB doesn't grow unbounded
  deleteTodo(todo.id);

  sleep(1);
}
