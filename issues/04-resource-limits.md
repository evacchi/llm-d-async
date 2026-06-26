### Problem Statement

The Helm deployment template does not set any `resources` (CPU/memory requests or limits) for the async-processor container, and `values.yaml` has no resource configuration section.

Without resource requests:
- The K8s scheduler cannot make informed placement decisions.
- The pod can be evicted under memory pressure with no warning.
- There is no protection against runaway memory usage (e.g. a queue backlog with large payloads).

Without resource limits:
- A single pod can starve other workloads on the node.
- Capacity planning is impossible.

### Proposed Solution

1. Add a `resources` section to `values.yaml` with sensible defaults (e.g. 100m/256Mi request, 500m/512Mi limit — to be tuned based on profiling).
2. Wire it into the deployment template.
3. Consider documenting sizing guidance based on concurrency settings and queue backend.

### Alternatives Considered

- Leaving resource configuration entirely to the user via `values.yaml` overrides — but having no defaults means out-of-the-box deployments are unbounded.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- batch-gateway's chart includes resource configuration for all three components (apiserver, processor, gc).
- llm-d-inference-scheduler chart includes resource defaults.
- Graduation criteria addressed: Deployment (production-readiness), Documentation.
