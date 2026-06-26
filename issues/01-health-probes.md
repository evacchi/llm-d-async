### Problem Statement

The async-processor deployment has no `livenessProbe`, `readinessProbe`, or `startupProbe` configured in the Helm chart (`charts/async-processor/templates/ap-deployments.yaml`). The Go binary (`cmd/main.go`) does not expose any health endpoint.

Without probes, Kubernetes cannot detect when the processor is hung (e.g. blocked on a dead Redis connection), has not yet finished initializing, or is in the process of draining. This means:

- A hung pod continues receiving work from the queue but never completes it.
- Rolling updates have no way to know when the new pod is ready.
- The `terminationGracePeriodSeconds: 130` in the chart is the only shutdown signal, but there is no corresponding graceful drain visible to K8s.

### Proposed Solution

1. Add an HTTP health server (e.g. on a configurable port, default `:8081`) exposing:
   - `/healthz` (liveness) — process is alive and not deadlocked.
   - `/readyz` (readiness) — queue connections established, worker pool running.
2. Add `livenessProbe`, `readinessProbe`, and optionally `startupProbe` to the Helm deployment template.
3. Add corresponding `values.yaml` entries for probe configuration (port, paths, intervals).

### Alternatives Considered

- Using the existing metrics port (9090) for probes — but mixing probes with metrics complicates auth configuration (`--metrics-endpoint-auth`) and would require unauthenticated probe paths.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- batch-gateway has health and readiness endpoints (`internal/apiserver/health/health_handler.go`) with corresponding probes in its chart.
- llm-d-inference-scheduler has dedicated health checking (`cmd/epp/runner/health_test.go`).
- Graduation criteria addressed: Reliability, Deployment (production-readiness).
