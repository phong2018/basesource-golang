# Performance Test Guide

## What These Scripts Do

These scripts simulate many users hitting your API at the same time.
The goal is to answer three questions:

1. **Is the API working correctly under zero load?** → smoke test
2. **How does the API behave as traffic grows?** → load test
3. **At what point does the API break?** → stress test

---

## Prerequisites

### 1. Install k6

```bash
brew install k6
```

k6 is the load testing tool. It acts like hundreds of virtual users sending HTTP requests to your API simultaneously.

### 2. Install metrics-server in kind (one-time setup)

Your kind cluster needs this to enable `kubectl top` (CPU/memory monitoring) and HPA autoscaling.

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

kubectl patch deployment metrics-server -n kube-system \
  --type=json -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

kubectl rollout status deployment/metrics-server -n kube-system
```

### 3. Make sure the cluster is running

```bash
kubectl -n basesource get pods
```

All pods should show `Running` and `READY`. If not, deploy the cluster first (see `wiki/memory/task/992.kubernetes-guide.md`).

---

## Quick Start

```bash
# run everything: smoke → load → stress
bash wiki/memory/task/993.performance-test/scripts/run_all.sh
```

Or run only the parts you want:

```bash
# just confirm the environment works
bash wiki/memory/task/993.performance-test/scripts/run_all.sh --smoke-only

# smoke + load tests, skip the breaking-point stress test
bash wiki/memory/task/993.performance-test/scripts/run_all.sh --no-stress

# smoke + one specific load scenario
bash wiki/memory/task/993.performance-test/scripts/run_all.sh --scenario=write_heavy
bash wiki/memory/task/993.performance-test/scripts/run_all.sh --scenario=mixed
bash wiki/memory/task/993.performance-test/scripts/run_all.sh --scenario=read_only
```

---

## The Three Tests Explained

### 1. Smoke Test (`smoke_test.js`)

**What it does:**
Sends 1 virtual user through the full CRUD cycle for 30 seconds.
Each iteration: create a todo → get it → list all → update it → delete it.

**Why:**
Before sending 500 users, you first need to confirm the API is actually responding correctly.
If the smoke test fails, the environment has a problem — fix it before load testing.

**What you expect to see:**
```
✓ create: HTTP 201
✓ get: HTTP 200
✓ list: HTTP 200
✓ update: HTTP 200
✓ delete: HTTP 204

checks.........................: 100.00%
http_req_duration p(95): under 100ms
```

---

### 2. Load Test (`load_test.js`)

**What it does:**
Ramps virtual users up gradually across 3 separate scenarios, one after another.

**The ramp shape (same for all 3 scenarios):**

```
VUs
500 |            ████████
200 |        ████
 50 |    ████
 10 | ███
  0 |──────────────────────── time
     30s  1m   1m   1m  30s
```

| Stage     | VUs | Duration | What you are testing                |
|-----------|-----|----------|-------------------------------------|
| Warm up   | 10  | 30s      | Gentle start, JIT warmup            |
| Baseline  | 50  | 1 min    | Normal expected traffic             |
| Medium    | 200 | 1 min    | Moderately busy                     |
| High      | 500 | 1 min    | Heavy traffic, HPA should activate  |
| Cool down | 0   | 30s      | Let in-flight requests finish       |

**The 3 scenarios:**

| Scenario       | Traffic mix              | What it tests                            |
|----------------|--------------------------|------------------------------------------|
| `write_heavy`  | 100% POST /api/v1/todos  | DB write speed, outbox dual-write        |
| `mixed`        | 70% GET list + 30% POST  | Read/write contention (most realistic)  |
| `read_only`    | 100% GET /api/v1/todos   | Maximum read throughput                  |

They run back-to-back automatically (total ~13 minutes).

**Why these 3 scenarios matter for this project:**
Every POST/PUT/DELETE writes 3 rows in one DB transaction (the todo + outbox_events + outbox_deliveries). This is heavier than a normal CRUD API. The `write_heavy` scenario exposes this pressure directly. The `read_only` scenario shows how fast reads are when there is no write contention.

**Pass/fail thresholds:**
- P95 latency < 1 second overall
- Error rate < 5%

If these are breached, k6 exits with code 1 (run_all.sh will stop).

---

### 3. Stress Test (`stress_test.js`)

**What it does:**
Ramps VUs far beyond the load test ceiling to find the exact point where the system breaks.

```
VUs
1500 |                    ████
1000 |            ████████
 500 |        ████
 200 |    ████
  50 | ███
   0 |──────────────────────────── time
      30s  1m  1m  2m  1m  2m  1m 1m 30s
```

**Important:** There are no pass/fail thresholds here. The stress test is designed to *find* the breaking point, not pass. It will keep running even when error rates spike — that is expected and the whole point.

**50/50 write/read mix:** harsher than production, maximises DB pressure.

**What you are looking for:**
- At what VU count does P95 latency jump from hundreds of ms to seconds?
- At what VU count does the error rate start climbing above 0%?
- Does the API recover when you back off the load (cool-down phase)?

---

## What to Monitor While Tests Run

Open a second terminal and run these while the tests are running:

```bash
# watch pod CPU / memory in real time
kubectl top pods -n basesource --watch

# watch HPA scaling decisions
kubectl get hpa -n basesource --watch

# tail API logs to see latency per request
kubectl -n basesource logs -l app=basesource-api -f
```

### What HPA output looks like during the test

```
NAME               REFERENCE                    TARGETS   MINPODS   MAXPODS   REPLICAS
basesource-api-hpa Deployment/basesource-api    12%/70%   2         10        2
basesource-api-hpa Deployment/basesource-api    68%/70%   2         10        2
basesource-api-hpa Deployment/basesource-api    84%/70%   2         10        4   ← scaling up
basesource-api-hpa Deployment/basesource-api    91%/70%   2         10        6   ← scaling again
```

You will see replicas increase as CPU goes above 70%. This typically takes about 30 seconds to react — so during a sharp spike the API will be slow before new pods come up.

---

## Reading k6 Output

After each test, k6 prints a summary like this:

```
✓ create: HTTP 201 ........... 12430 / 12430 (100%)
✓ list: HTTP 200 ............. 28700 / 28700 (100%)

http_req_duration ........ avg=45ms   min=2ms  med=38ms  max=4.1s  p(90)=89ms  p(95)=134ms  p(99)=890ms
http_reqs ................ 41130 / 4.1s (RPS=174.3)
http_req_failed .......... 0.00%
```

| Field               | What it means                                              |
|---------------------|------------------------------------------------------------|
| `avg`               | Average response time across all requests                  |
| `p(95)`             | 95% of requests finished within this time — most important |
| `p(99)`             | 99% of requests — catches the slowest outliers             |
| `max`               | The single slowest request (often a spike, less reliable)  |
| `http_reqs / RPS`   | Total requests sent and requests per second achieved       |
| `http_req_failed`   | % of requests that got an error (4xx, 5xx, or timeout)     |

**Why P95 matters more than average:**
If average is 45ms but P95 is 2 seconds, it means 1 in 20 users waits 2 seconds. That is a bad user experience even though the average looks fine. Always judge by P95 or P99.

---

## Analyzing Server-Side After the Test

### Check latency from API logs

```bash
kubectl -n basesource logs -l app=basesource-api --tail=5000 | \
  grep '"msg":"request completed"' | jq '.latency_ms' | sort -n | \
  awk 'BEGIN{c=0;s=0} {c++;s+=$1} END{print "avg:", s/c, "ms  count:", c}'
```

### Check HPA scaling events

```bash
kubectl describe hpa basesource-api-hpa -n basesource
```

Look at the `Events` section at the bottom — it shows when pods were added and why.

### Check MySQL thread connections

```bash
kubectl -n basesource exec \
  $(kubectl -n basesource get pod -l app=mysql -o jsonpath='{.items[0].metadata.name}') \
  -- mysql -u appuser -papppass appdb -e "SHOW STATUS LIKE 'Threads_connected';"
```

If this number is close to the MySQL `max_connections` limit (default 151), the database connection pool is saturated — a common bottleneck.

---

## Recording Your Results

After running the load test, fill in this table (copy to the plan document):

| Stage    | VUs | RPS achieved | P50 | P95 | P99 | Error rate |
|----------|-----|--------------|-----|-----|-----|------------|
| Warm up  | 10  |              |     |     |     |            |
| Baseline | 50  |              |     |     |     |            |
| Medium   | 200 |              |     |     |     |            |
| High     | 500 |              |     |     |     |            |

For the stress test, note at which VU count:
- P95 crossed 1 second: **_____ VUs**
- Error rate first appeared: **_____ VUs**
- System became unstable: **_____ VUs**

---

## Common Problems and Fixes

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| `kubectl top` says `Metrics API not available` | metrics-server not installed | Run the Step 0 install commands |
| HPA never scales up during load test | metrics-server not installed | Same as above |
| Smoke test fails with `connection refused` | Port-forward not running | run_all.sh starts it automatically; run smoke manually only after confirming port-forward is live |
| High error rate at low VUs (< 50) | API or DB not ready | `kubectl -n basesource get pods` — check all pods are Running |
| P95 is very high (>5s) even at baseline 50 VUs | kind cluster is CPU-starved | Reduce Docker Desktop CPU allocation, close other apps |
| MySQL `Threads_connected` near 151 | Connection pool exhausted | This is the bottleneck — note it in results |

---

## File Reference

```
scripts/
├── run_all.sh        ← start here
├── smoke_test.js     ← 1 VU, 30s, full CRUD, confirms env works
├── load_test.js      ← 3 scenarios, gradual ramp, finds safe limit
├── stress_test.js    ← pushes to 1500 VUs, finds breaking point
└── lib/
    └── helpers.js    ← shared HTTP call functions (not run directly)
```
