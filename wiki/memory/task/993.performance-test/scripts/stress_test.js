// stress_test.js — push beyond normal capacity to find the breaking point
//
// Strategy: ramp VUs steeply past the load_test ceiling, hold at each plateau
// long enough to observe whether error rate or latency stabilises or keeps climbing.
// No thresholds — the goal is to *observe*, not pass/fail.
//
// Run:
//   k6 run wiki/memory/task/993.performance-test/scripts/stress_test.js
//
// What to watch while running:
//   kubectl top pods -n basesource --watch
//   kubectl get hpa -n basesource --watch
//   kubectl -n basesource logs -l app=basesource-api -f
import { sleep } from 'k6';
import { createTodo, listTodos, randomTitle } from './lib/helpers.js';

export const options = {
  scenarios: {
    stress: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 50   },  // warm up to baseline
        { duration: '1m',  target: 200  },  // medium — should be stable
        { duration: '1m',  target: 500  },  // high   — HPA starts scaling
        { duration: '2m',  target: 500  },  // hold high — watch if it stabilises
        { duration: '1m',  target: 1000 },  // stress — expect degradation
        { duration: '2m',  target: 1000 },  // hold stress — find the limit
        { duration: '1m',  target: 1500 },  // spike — likely breaking point
        { duration: '1m',  target: 1500 },  // hold spike
        { duration: '30s', target: 0    },  // cool down
      ],
      gracefulRampDown: '30s',
    },
  },

  // no pass/fail thresholds — stress test is for observation only
  // but still collect tagged metrics for analysis
  summaryTrendStats: ['min', 'med', 'avg', 'p(90)', 'p(95)', 'p(99)', 'max', 'count'],
};

// Mix of write (heavier, outbox write) and read to simulate realistic load.
// 50/50 write-read is harsher than production but exercises both paths.
export default function () {
  if (Math.random() < 0.5) {
    createTodo(randomTitle('stress'));
  } else {
    listTodos();
  }
  // no sleep — maximum pressure
}
