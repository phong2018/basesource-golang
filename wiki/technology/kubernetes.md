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

---

## Interview Questions — Foundation to Advanced

### Foundation

**Q1: What is Kubernetes and why do we need it?**
> Kubernetes (K8s) is a container orchestration platform that automates deployment, scaling, and lifecycle management of containerized applications. You need K8s when you have many containers that must run together, need automatic restart on crash, scale with traffic, and zero-downtime rolling updates.

**Q2: What is a Pod? Why not deploy containers directly?**
> A Pod is the smallest deployable unit in K8s, containing one or more containers that share a network namespace and storage volumes. K8s does not manage containers directly because a Pod provides: shared localhost networking between containers, shared volumes, and a unified lifecycle unit. Example: an app container and a log sidecar that communicate over localhost.

**Q3: What is the difference between Deployment, StatefulSet, and DaemonSet?**
> - **Deployment**: stateless apps, pods are interchangeable (any pod = any pod), rolling update. Use for API servers.
> - **StatefulSet**: stateful apps that need stable identity (pod-0, pod-1), stable storage, ordered startup/shutdown. Use for databases, Kafka, Zookeeper.
> - **DaemonSet**: runs exactly one pod per node. Use for log collectors (Fluentd), monitoring agents (Prometheus node-exporter).

**Q4: What is a Service? What types of Services exist?**
> A Service provides a stable IP/DNS for a group of pods (selector-based). Types:
> - **ClusterIP** (default): only accessible inside the cluster. Use for internal communication.
> - **NodePort**: exposes a port on every node (30000–32767). Use for local testing.
> - **LoadBalancer**: provisions a cloud load balancer (AWS ALB, GCP LB). Use for production external traffic.
> - **ExternalName**: maps a service to an external DNS name (CNAME).

**Q5: ConfigMap vs Secret — what is the difference?**
> - **ConfigMap**: stores non-sensitive config (ports, feature flags, topic names). Stored as plaintext in etcd. Safe to version-control the template.
> - **Secret**: stores sensitive data (passwords, tokens, certs). Base64-encoded (not encrypted by default). Requires encryption at rest + RBAC for real protection.
> - Both are injected into pods via `env`, `envFrom`, or volume mount.

**Q6: What is a Namespace used for?**
> A Namespace is a virtual cluster inside a physical cluster — used to isolate resources by team, environment (dev/staging/prod), or application. RBAC, NetworkPolicy, and ResourceQuota all apply per namespace.

**Q7: kubectl get, describe, logs, exec — when do you use each?**
> - `get`: view resource status (name, status, age)
> - `describe`: view detail + events of a resource (debug crashloop, probe failure)
> - `logs`: view stdout/stderr of a container
> - `exec -it pod -- sh`: get a shell inside a container for live debugging
> - `port-forward`: forward a local port → pod port without exposing a Service

---

### Intermediate

**Q8: Liveness probe vs Readiness probe — what is the difference?**
> - **Readiness probe**: determines whether the pod is ready to receive traffic. Failure → pod is removed from Service endpoints. Use when the app needs warmup time (cache load, DB connection).
> - **Liveness probe**: determines whether the pod is still alive. Failure → kubelet restarts the container. Use when the app can deadlock without exiting.
> - **Startup probe**: gives slow-starting apps extra time before liveness/readiness checks begin.
>
> Common mistake: using a liveness probe to check an external dependency (DB, Redis) → cascading restarts when the DB goes down.

**Q9: How does a rolling update work? What are maxUnavailable and maxSurge?**
> A rolling update replaces old pods with new pods incrementally:
> - `maxUnavailable`: maximum number of pods that can be unavailable during the update. `0` = never reduce capacity.
> - `maxSurge`: maximum number of extra pods that can be created beyond the replica count. `1` = create one new pod first, then delete an old one.
>
> This project's API uses `maxUnavailable:0, maxSurge:1` — guarantees at least 2 healthy pods exist before the old pod is removed.

**Q10: How does HPA (Horizontal Pod Autoscaler) work?**
> HPA periodically (default every 15s) queries the metrics server → compares to the target → calculates the desired replica count:
> ```
> desiredReplicas = ceil(currentReplicas × (currentMetric / desiredMetric))
> ```
> Example: 2 pods, CPU at 90%, target 70% → ceil(2 × 90/70) = 3 pods.
> Scale-down has a cooldown period (default 5 minutes) to prevent flapping.
> Requires **metrics-server** deployed in the cluster.

**Q11: What are PersistentVolume and PersistentVolumeClaim?**
> - **PersistentVolume (PV)**: a storage resource in the cluster (AWS EBS, GCP PD, NFS). Exists independently of pod lifecycle.
> - **PersistentVolumeClaim (PVC)**: a storage request from a user. K8s binds the PVC to a matching PV (size, access mode, storage class).
> - **StorageClass**: defines how to provision storage dynamically. When a PVC references a StorageClass, K8s automatically provisions a PV.
>
> Access modes: `ReadWriteOnce` (1 node), `ReadOnlyMany` (many nodes read), `ReadWriteMany` (many nodes read+write, requires shared storage like NFS/EFS).

**Q12: What is an Init Container? How does it differ from a Sidecar?**
> - **Init container**: runs before the main containers and must complete (exit 0) before the main container starts. Used for: running migrations, waiting for a dependency to be ready, cloning config from git.
> - **Sidecar container**: runs alongside the main container in the same pod. Used for: log shipping, service mesh proxy (Envoy), secret rotation.
>
> This project uses a Job instead of an init container for migrations — allowing independent reruns.

**Q13: What are resource requests vs limits? Why do they matter?**
> - **requests**: the amount of CPU/memory the scheduler uses to place a pod on a node. The pod is guaranteed these resources.
> - **limits**: the ceiling — the pod cannot exceed this. Exceeding CPU limit → throttled. Exceeding memory limit → OOMKilled.
>
> No requests set → the scheduler has no information to place the pod, risks node overload.
> No limits set → a single pod can consume all resources on a node.
> QoS classes: `Guaranteed` (requests = limits), `Burstable` (requests < limits), `BestEffort` (neither set).

**Q14: How does RBAC work in Kubernetes?**
> Consists of 4 objects:
> - **ServiceAccount**: the identity for a pod or process
> - **Role / ClusterRole**: a set of permissions (verbs: get, list, create, delete on resources: pods, secrets…)
> - **RoleBinding / ClusterRoleBinding**: binds a Role to a Subject (ServiceAccount, User, Group)
>
> `Role` + `RoleBinding` → scoped to a namespace.
> `ClusterRole` + `ClusterRoleBinding` → cluster-wide.

---

### Advanced

**Q15: What is etcd and why is it critical?**
> etcd is a distributed key-value store that holds the entire cluster state (pod specs, service configs, secrets). If etcd is lost, the cluster loses all state. Requirements:
> - Regular etcd backups (`etcdctl snapshot save`)
> - Run an etcd cluster (3 or 5 nodes) for quorum-based fault tolerance
> - Enable encryption at rest for secrets in etcd
>
> Control plane components: etcd + API server + scheduler + controller manager.

**Q16: Explain pod scheduling — how is a node selected?**
> The scheduler runs in two phases:
> 1. **Filtering**: eliminates nodes that do not satisfy requirements — `nodeSelector`, `taints/tolerations`, resource requests, `affinity` rules.
> 2. **Scoring**: ranks remaining nodes — least-requested, image locality, spread…
>
> Scheduling control tools:
> - `nodeSelector`: select a node by a simple label match
> - `nodeAffinity`: select a node with more complex label logic (preferred/required)
> - `podAntiAffinity`: ensure pods of the same Deployment do not land on the same node/zone — high availability
> - `taints + tolerations`: nodes repel pods; pods opt-in by tolerating the taint (dedicated nodes)

**Q17: What is a NetworkPolicy? What is the default behavior?**
> By default K8s allows ALL traffic between all pods in the cluster (flat network).
> A NetworkPolicy is a set of firewall rules for pods — uses label selectors to control ingress and egress.
>
> Example: allow only API pods to connect to the MySQL pod, deny all other traffic to MySQL:
> ```yaml
> spec:
>   podSelector:
>     matchLabels: {app: mysql}
>   ingress:
>   - from:
>     - podSelector:
>         matchLabels: {app: basesource-api}
>     ports:
>     - port: 3306
> ```
> Note: requires a CNI plugin that supports NetworkPolicy (Calico, Cilium, Weave). Flannel does not.

**Q18: What problems does a Service Mesh (Istio/Linkerd) solve that K8s does not?**
> A K8s Service only does L4 load balancing (TCP/UDP). A Service Mesh adds:
> - **mTLS**: automatically encrypts and authenticates traffic between services
> - **L7 routing**: route by HTTP headers, canary deployments (5% traffic → new version)
> - **Observability**: distributed tracing, per-service-call metrics
> - **Retry / circuit breaker**: automatically retry failed requests, circuit-break an unhealthy service
>
> Trade-offs: added latency (sidecar proxy), operational complexity, resource overhead.

**Q19: Compare rolling update vs blue/green vs canary deployment.**
> - **Rolling update**: gradually replaces old pods. Simple, K8s-native. Downside: during the update, both old and new versions run simultaneously (backward compatibility required).
> - **Blue/Green**: deploy the entire new version (green), switch traffic by updating the Service selector. Instant rollback by switching back. Costs double the resources.
> - **Canary**: route a small percentage of traffic (5–10%) to the new version, monitor, then gradually increase. Requires ingress or service mesh for weight-based routing. Detects bugs early before they affect all users.

**Q20: Voluntary vs involuntary pod disruption — how do you handle each?**
> - **Voluntary disruption**: node drain (maintenance), rolling update, HPA scale-down. Controlled — use a **PodDisruptionBudget (PDB)** to limit how many pods can be removed simultaneously.
> - **Involuntary disruption**: node crash, OOMKill, hardware failure. Uncontrollable — use multiple replicas + pod anti-affinity to spread across nodes/zones.
>
> PDB example: ensure at least 1 pod is always running during a node drain:
> ```yaml
> spec:
>   minAvailable: 1
>   selector:
>     matchLabels: {app: basesource-api}
> ```

**Q21: How do you debug a pod stuck in CrashLoopBackOff?**
> Debug workflow:
> ```bash
> kubectl describe pod <pod>      # check Events: exit code, OOMKilled, probe failure
> kubectl logs <pod> --previous   # logs from the previous crashed run
> kubectl logs <pod> -f           # stream logs in real-time
> ```
> Common causes:
> - Exit code 1: application error → check logs
> - Exit code 137 (128+9): OOMKilled → increase memory limit
> - Exit code 139: segfault
> - Liveness probe failure: probe too aggressive or app slow to start → add `initialDelaySeconds`
> - Image pull error: wrong tag, or missing `imagePullSecret` for a private registry

**Q22: What are Secrets management best practices in production?**
> K8s Secrets are only base64-encoded by default, not encrypted. Best practices:
> 1. **Encryption at rest**: enable `EncryptionConfiguration` in the API server to encrypt secrets in etcd
> 2. **External secret manager**: use HashiCorp Vault, AWS Secrets Manager, or GCP Secret Manager with ESO (External Secrets Operator) to sync secrets into K8s
> 3. **RBAC**: restrict who can `get`/`list` secrets — principle of least privilege
> 4. **Audit logging**: log all access to secrets
> 5. **Avoid env vars**: mount secrets as a volume (file) rather than env vars — env vars can leak through crash dumps
> 6. **Rotate regularly**: implement a process to rotate secrets periodically

**Q23: Explain CNI (Container Network Interface) and why it is needed.**
> K8s defines a network model but does not implement it. A CNI plugin implements:
> - Each pod gets a unique IP routable within the cluster
> - Pods on the same node communicate directly
> - Pods on different nodes communicate through an overlay network (VXLAN, BGP…)
>
> Popular CNI plugins:
> - **Flannel**: simple, VXLAN overlay, no NetworkPolicy support
> - **Calico**: BGP routing, NetworkPolicy support, good performance
> - **Cilium**: eBPF-based, L7 NetworkPolicy, built-in observability, high performance

**Q24: Explain how Kubernetes handles graceful shutdown.**
> When a pod is terminated (rolling update, scale-down, eviction):
> 1. Pod state → `Terminating`; removed from Service endpoints (no new traffic)
> 2. `preStop` hook runs (if configured)
> 3. SIGTERM is sent to the container process
> 4. K8s waits for `terminationGracePeriodSeconds` (default 30s)
> 5. If the process has not exited → SIGKILL
>
> The application must handle SIGTERM to finish in-flight requests. In Go:
> ```go
> quit := make(chan os.Signal, 1)
> signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
> <-quit
> ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
> defer cancel()
> server.Shutdown(ctx)
> ```

**Q25: StatefulSet vs Deployment — when do you choose StatefulSet?**
> Choose StatefulSet when the app needs:
> - **Stable network identity**: pod-0, pod-1 — DNS `pod-0.service.namespace.svc.cluster.local` does not change on restart
> - **Stable storage**: each pod has its own PVC, not shared, not reassigned when the pod reschedules
> - **Ordered startup/shutdown**: pod-0 must be ready before pod-1 starts (critical for database clusters — primary before replica)
> - **Ordered rolling updates**: pods updated in reverse order (pod-N first)
>
> Examples: MySQL primary/replica, Kafka (broker-0/1/2), Zookeeper, Elasticsearch nodes.
> Stateless API server → Deployment (pods are interchangeable).

**Q26: When do you use Job vs CronJob vs Deployment?**
> - **Job**: a task that runs to completion and does not restart. `completions: 1, backoffLimit: 3`. Examples: DB migration, data import, one-time batch processing.
> - **CronJob**: a Job that runs on a cron schedule. Examples: cleanup job at 2am, daily report generation.
> - **Deployment**: a long-running service that must always be available. Restarts on crash.
>
> Job parallelism: `parallelism: 3` runs 3 pods concurrently to process a work queue.

**Q27: Explain the Kubernetes control plane components and the role of each.**
> - **API Server**: the single entry point into the cluster. Validates and persists resources to etcd. All components communicate through the API server.
> - **etcd**: distributed store holding cluster state. The single source of truth.
> - **Scheduler**: watches pods that have no node assigned and selects the best node based on resources and constraints.
> - **Controller Manager**: runs controllers (Deployment controller, ReplicaSet controller, Job controller…) — continuously reconciles actual state toward desired state.
> - **Cloud Controller Manager**: integrates with cloud provider APIs (creates LBs, attaches EBS volumes…).
>
> Worker node components: **kubelet** (node agent, runs pods), **kube-proxy** (network rules), **container runtime** (containerd/CRI-O).

**Q28: Why does Kubernetes use a declarative model instead of imperative?**
> **Imperative**: "run 3 replicas" — a one-time command with no ongoing reconciliation.
> **Declarative**: "desired state = 3 replicas" — a controller continuously reconciles actual state toward the desired state.
>
> Benefits of the declarative model:
> - **Self-healing**: if a pod crashes, the controller recreates it automatically without manual intervention
> - **GitOps**: store desired state in git; `kubectl apply` = deploy
> - **Idempotent**: applying the same manifest multiple times produces the same result
> - **Audit trail**: diff manifests to understand exactly what changed and when

**Q29: What is the difference between ResourceQuota and LimitRange?**
> - **ResourceQuota**: caps the total resources for an entire namespace. Example: namespace `dev` cannot use more than 10 CPU cores, 20Gi memory, or create more than 50 pods.
> - **LimitRange**: sets defaults and min/max per pod/container within a namespace. If a pod does not set requests/limits, LimitRange injects the defaults.
>
> Use both together: ResourceQuota prevents a dev team from consuming the entire cluster; LimitRange ensures every pod has requests set so the scheduler can place it correctly.

**Q30: Explain Ingress and why an Ingress Controller is needed.**
> **Ingress**: a K8s resource that defines rules for routing HTTP/HTTPS traffic into the cluster (host-based routing, path-based routing, TLS termination).
> **Ingress Controller**: the component that implements the Ingress rules (nginx, Traefik, AWS ALB Ingress Controller). K8s does not ship a built-in controller.
>
> Example routing:
> ```
> api.example.com/api/v1  → basesource-api-svc:80
> admin.example.com       → basesource-admin-svc:80
> ```
> TLS is terminated at the Ingress — traffic inside the cluster can be plain HTTP (or mTLS if a service mesh is used).
> cert-manager integrates with Ingress to auto-renew Let's Encrypt certificates.
