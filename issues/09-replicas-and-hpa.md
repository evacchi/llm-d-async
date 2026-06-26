### Problem Statement

The deployment template hardcodes `replicas: 1`. There is no `values.yaml` knob for replica count and no `HorizontalPodAutoscaler` or `PodDisruptionBudget` template.

A single replica is a single point of failure — if the pod is evicted or crashes, queue processing stops entirely until it restarts. The sorted-set and GCP Pub/Sub backends both support competing consumers natively (atomic `ZPOPMIN` and subscription-level load balancing respectively), so horizontal scaling is safe for those backends. The two-phase graceful shutdown (PR #238) ensures in-flight requests drain cleanly on rolling updates.

The `redis-pubsub` backend is broadcast-based and does **not** support multiple replicas — each subscriber receives every message, causing duplicate processing.

### Proposed Solution

1. Make `replicas` configurable in `values.yaml` (default: 1).
2. Add an optional `HorizontalPodAutoscaler` template gated by `autoscaling.enabled`, with configurable min/max replicas and target CPU/memory utilization.
3. Add an optional `PodDisruptionBudget` template gated by `pdb.enabled` with configurable `minAvailable` or `maxUnavailable`.
4. Document which backends support horizontal scaling (`redis-sortedset`, `gcp-pubsub`) and which do not (`redis-pubsub`). Consider adding a chart-level validation that warns or fails when `replicas > 1` with `redis-pubsub`.

### Alternatives Considered

- Running multiple single-replica deployments against separate queues for isolation — works but doesn't help with HA for a single queue.
- Adding in-flight tracking with heartbeats (as batch-gateway does) — over-engineering for the async-processor's use case, where the cost of replaying a single inference request on hard crash is low.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- Depends on two-phase graceful shutdown (PR #238 / issue #227) being merged to ensure clean rolling updates.
- The `redis-pubsub` backend is already documented as non-production ("consider using sorted-set for production").
- Graduation criteria addressed: Reliability (high availability), Deployment (production-readiness).
