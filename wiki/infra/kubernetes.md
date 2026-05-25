# Kubernetes Deployment Architecture

This document describes how the basesource application is structured and deployed on Kubernetes.

---

## Cluster Layout

```
kind cluster: basesource
└── namespace: basesource
    ├── Infra
    │   ├── mysql          (Deployment + Service)
    │   ├── rabbitmq       (Deployment + Service)
    │   └── kafka          (Deployment + Service)
    ├── App
    │   ├── basesource-api     (Deployment × 2 pods + Service + Ingress + HPA)
    │   └── basesource-worker  (Deployment × 2 pods + HPA)
    ├── Config
    │   ├── basesource-config  (ConfigMap)
    │   └── basesource-secret  (Secret)
    └── Jobs
        └── basesource-migrate (Job — runs once)
```

---

## Traffic Flow

```
User
 │
 ▼
Ingress (nginx)          api.example.com → basesource-api-svc:80
 │
 ▼
Service (ClusterIP)      round-robin across healthy API pods
 │
 ├──► API Pod 1 (:8080)
 └──► API Pod 2 (:8080)
         │
         ▼
      MySQL (appdb)      write todo + outbox_events + outbox_deliveries
```

```
Worker Pod 1 / Pod 2  — 4 independent goroutines via errgroup
 │
 ├── KafkaOutboxRelay           polls outbox_deliveries → publishes to Kafka topic (todo-events)
 │                                                                │
 │                                                                ▼
 ├── KafkaConsumer              reads todo-events topic → DomainEventHandler (audit log / analytics)
 │
 ├── RabbitMQOutboxRelay        polls outbox_deliveries → publishes to RabbitMQ exchange (todo.events)
 │                                                                │
 │                                                                ▼ routed to todo.notifications queue
 └── RabbitMQNotificationConsumer  reads todo.notifications queue → HandleNotificationTask (email / push)
```

---

## Manifests

```
k8s/
├── namespace.yaml          namespace: basesource
├── configmap.yaml          non-secret env vars (ports, topic names, exchange)
├── secret.yaml             template — CHANGE_ME placeholders
├── secret.local.yaml       local kind values (do not commit prod credentials)
├── migrate-job.yaml        one-shot Job: runs `app migrate`
├── infra/
│   ├── mysql.yaml          Deployment + Service
│   ├── rabbitmq.yaml       Deployment + Service
│   └── kafka.yaml          Deployment + Service
├── api/
│   ├── deployment.yaml     2 replicas, rolling update (maxUnavailable:0, maxSurge:1)
│   ├── service.yaml        ClusterIP, port 80 → 8080
│   ├── ingress.yaml        nginx + cert-manager TLS
│   └── hpa.yaml            min:2, max:10, target 70% CPU
└── worker/
    ├── deployment.yaml     2 replicas, rolling update (maxUnavailable:1, maxSurge:1)
    └── hpa.yaml            min:2, max:6, target 70% CPU
```

---

## Key Design Decisions

### API — Zero-Downtime Rolling Update

```yaml
strategy:
  rollingUpdate:
    maxUnavailable: 0   # never drop below 2 ready pods
    maxSurge: 1         # spin up a 3rd pod first, then kill an old one
```

New pod must pass the readiness probe (`GET /health`) before traffic is shifted.  
Old pod receives a SIGTERM and has 30 seconds to drain in-flight requests.

### Worker — No Liveness Probe

The worker image is `distroless/static-debian11:nonroot` — no shell, no `pgrep`.  
A liveness probe requiring shell commands would always fail.

Instead, the worker uses `errgroup`: if any goroutine (relay, consumer) returns an error,
the whole process exits. Kubernetes detects the exit and restarts the pod automatically.

```yaml
# worker deployment.yaml has NO livenessProbe — intentional
```

### Readiness vs Liveness

| Probe | API | Worker |
|---|---|---|
| Readiness | `GET /health` — gates traffic routing | none (worker doesn't serve HTTP) |
| Liveness | `GET /health` — restarts hung pods | none (process exit handles it) |

### ConfigMap vs Secret

| Key | Where |
|---|---|
| `APP_PORT`, `KAFKA_TOPIC`, `RABBITMQ_EXCHANGE` | ConfigMap — safe to version control |
| `DATABASE_DSN`, `RABBITMQ_URL`, `KAFKA_BROKERS`, `AWS_*` | Secret — never in git |

Both are injected via `envFrom` — the app reads them as normal environment variables.

### HPA Scaling Boundaries

| Deployment | Min | Max | Scale trigger |
|---|---|---|---|
| basesource-api | 2 | 10 | CPU > 70% |
| basesource-worker | 2 | 6 | CPU > 70% |

Minimum 2 replicas ensures high availability — one pod can be restarted without downtime.

---

## Image Strategy

The binary is built from `golang:1.25` and copied into `distroless/static-debian11:nonroot`.

```dockerfile
FROM golang:1.25 AS builder
RUN CGO_ENABLED=0 GOOS=linux go build -a -o app .

FROM gcr.io/distroless/static-debian11:nonroot AS final
COPY --from=builder /src/app /usr/local/bin/app
```

Benefits of distroless:
- No shell, no package manager — drastically smaller attack surface.
- Image is ~10 MB vs ~800 MB for golang:1.25.
- `nonroot` variant runs as uid 65532 by default — matches `runAsUser: 65532` in pod spec.

For **local kind**, the image is loaded directly:
```bash
docker build -f docker/Dockerfile -t basesource:local .
kind load docker-image basesource:local --name basesource
```
`imagePullPolicy: Never` tells kubelet to use the pre-loaded image, not try to pull from a registry.

---

## Database Migrations

Migrations run as a Kubernetes **Job**, not as an init container or startup hook.

```
kubectl apply -f k8s/migrate-job.yaml
kubectl -n basesource wait --for=condition=complete job/basesource-migrate --timeout=60s
```

A Job runs exactly once (with `backoffLimit: 3` on failure), then stays in `Completed` state.
This separates migration from deployment — you can rerun the job safely if it fails.

---

## Deploy Order

Dependencies must be ready before the next step:

```
1. namespace
2. configmap + secret
3. infra (mysql, rabbitmq, kafka)       ← wait for readiness
4. migrate-job                          ← wait for complete
5. api + worker                         ← wait for readiness
```

Full deploy script:
```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml -f k8s/secret.local.yaml
kubectl apply -f k8s/infra/
kubectl -n basesource wait --for=condition=ready pod -l app=mysql --timeout=120s
kubectl -n basesource wait --for=condition=ready pod -l app=rabbitmq --timeout=120s
kubectl -n basesource wait --for=condition=ready pod -l app=kafka --timeout=120s
kubectl apply -f k8s/migrate-job.yaml
kubectl -n basesource wait --for=condition=complete job/basesource-migrate --timeout=60s
kubectl apply -f k8s/api/ -f k8s/worker/
kubectl -n basesource get pods
```

---

## Hands-On Learning

See [992.kubernetes-guide.md](../memory/task/992.kubernetes-guide.md) for step-by-step labs:
killing pods, rolling updates, scaling, log tailing, exec into containers, and rebuilding images.
