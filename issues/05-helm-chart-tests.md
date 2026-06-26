### Problem Statement

The `charts/async-processor/` directory has no `tests/` subdirectory and no chart tests. The CI pipeline runs `helm lint` but does not validate that rendered manifests are correct for various value combinations.

Given the complex conditional logic in `ap-deployments.yaml` (Redis sorted-set vs pubsub vs GCP Pub/Sub, single-queue vs multi-queue, gate type validation, TLS mounts, OTel env vars), untested value combinations can produce invalid manifests silently.

### Proposed Solution

1. Add `charts/async-processor/tests/` with test cases covering:
   - Redis pubsub single-queue
   - Redis sorted-set with per-queue gates
   - GCP Pub/Sub mode
   - OTel enabled vs disabled
   - TLS mount presence/absence
   - Invalid combo rejection (`queuesConfig` + `gateType`)
   - Metrics port and auth settings
2. Add `helm unittest` to the CI pipeline (alongside `helm lint`).

### Alternatives Considered

- Relying solely on `helm lint` and E2E tests to catch template errors — but lint only checks syntax, and E2E tests only exercise one value combination per run.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- batch-gateway has 6 chart test files covering configmap rendering, deployment, httproute, and observability resources, using the `helm-unittest` plugin.
- Graduation criteria addressed: Test coverage, Deployment (production-readiness).
