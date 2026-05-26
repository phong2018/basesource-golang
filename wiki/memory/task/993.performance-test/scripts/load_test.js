// load_test.js — main load test: gradual ramp across 3 scenarios
//
// Scenario A (write_heavy):  100% POST — exercises DB write + outbox dual-write
// Scenario B (mixed):        70% GET list + 30% POST — realistic read/write mix
// Scenario C (read_only):    100% GET list — maximum read throughput baseline
//
// Run all:
//   k6 run wiki/memory/task/993.performance-test/scripts/load_test.js
//
// Run one scenario:
//   k6 run --env SCENARIO=write_heavy wiki/memory/task/993.performance-test/scripts/load_test.js
//   k6 run --env SCENARIO=mixed       wiki/memory/task/993.performance-test/scripts/load_test.js
//   k6 run --env SCENARIO=read_only   wiki/memory/task/993.performance-test/scripts/load_test.js
import { sleep } from 'k6';
import { createTodo, listTodos, getTodo, randomTitle } from './lib/helpers.js';

// ── stage shape (shared across all scenarios) ────────────────────────────────
const STAGES = [
  { duration: '30s', target: 10  },  // warm up
  { duration: '1m',  target: 50  },  // baseline  — target: P95 < 200ms
  { duration: '1m',  target: 200 },  // medium    — target: P95 < 500ms
  { duration: '1m',  target: 500 },  // high      — target: P95 < 1s
  { duration: '30s', target: 0   },  // cool down
];

// ── scenario filter ───────────────────────────────────────────────────────────
const ONLY = __ENV.SCENARIO || 'all';

function scenarioEnabled(name) {
  return ONLY === 'all' || ONLY === name;
}

// ── options ───────────────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    ...(scenarioEnabled('write_heavy') && {
      write_heavy: {
        executor: 'ramping-vus',
        stages: STAGES,
        startVUs: 0,
        gracefulRampDown: '10s',
        exec: 'writeHeavy',
      },
    }),
    ...(scenarioEnabled('mixed') && {
      mixed: {
        executor: 'ramping-vus',
        stages: STAGES,
        startVUs: 0,
        gracefulRampDown: '10s',
        startTime: scenarioEnabled('write_heavy') ? '4m30s' : '0s',  // run after write_heavy
        exec: 'mixedReadWrite',
      },
    }),
    ...(scenarioEnabled('read_only') && {
      read_only: {
        executor: 'ramping-vus',
        stages: STAGES,
        startVUs: 0,
        gracefulRampDown: '10s',
        startTime: scenarioEnabled('mixed') ? '9m' : scenarioEnabled('write_heavy') ? '4m30s' : '0s',
        exec: 'readOnly',
      },
    }),
  },
  thresholds: {
    // overall across all scenarios
    http_req_duration:                  ['p(95)<1000'],
    http_req_failed:                    ['rate<0.05'],
    // per-scenario thresholds
    'http_req_duration{scenario:write_heavy}': ['p(95)<1000'],
    'http_req_duration{scenario:mixed}':       ['p(95)<1000'],
    'http_req_duration{scenario:read_only}':   ['p(95)<500'],
  },
};

// ── Scenario A: write-heavy (100% POST) ──────────────────────────────────────
export function writeHeavy() {
  createTodo(randomTitle('load-write'));
  sleep(0.1);
}

// ── Scenario B: mixed (70% GET list, 30% POST) ───────────────────────────────
export function mixedReadWrite() {
  if (Math.random() < 0.7) {
    listTodos();
  } else {
    createTodo(randomTitle('load-mixed'));
  }
  sleep(0.1);
}

// ── Scenario C: read-only (100% GET list) ────────────────────────────────────
export function readOnly() {
  listTodos();
  sleep(0.05);
}
