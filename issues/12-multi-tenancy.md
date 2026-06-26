# Clarify and strengthen multi-tenancy story

## Problem

The async-processor supports multi-queue configurations with per-queue dispatch gates, inference objectives, and routing. The `redis-quota` gate supports per-attribute rate limiting (e.g. by `userid`). However:

1. **No tenant isolation at the namespace level.** A single deployment processes all queues. There is no guidance on deploying per-tenant instances vs shared instances with per-queue isolation.
2. **No RBAC scoping.** The ServiceAccount has no documented RBAC needs, but the metrics endpoint supports authentication (`--metrics-endpoint-auth`). There is no guidance on restricting which tenants can access which queues.
3. **No resource quota or fairness enforcement.** The `redis-quota` gate provides per-user rate limiting within a queue, but there is no mechanism to enforce fairness across queues (e.g. one tenant's queue consuming all worker capacity).
4. **The `classifying` mode for `redis-quota`** tags messages as "reserved" or "overflow" but there is no documented pattern for what a downstream system should do with overflow messages.

## Context

- batch-gateway has explicit multi-tenant E2E tests (`test/e2e/multitenant_test.go`).
- The well-lit path doc mentions "filling slack capacity" which implies shared infrastructure with multiple workload types.
- The `InferenceObjective` header provides workload-level routing, but tenant-level isolation is the user's responsibility with no guidance.

## Proposed work

1. Document the multi-tenancy model:
   - Shared instance with per-queue isolation (when appropriate, limitations).
   - Per-tenant instance deployment pattern (namespace isolation).
   - How `redis-quota` + `composite` gates provide per-tenant fairness.
2. Add a multi-tenant E2E test scenario (multiple queues with different quota settings, verify fairness).
3. Consider adding per-queue worker pool sizing (currently all queues share the same `concurrency` pool).
4. Document the `classifying` mode pattern with a concrete example.

## Graduation criteria addressed

- Multi-tenancy
- Documentation
- Tested by users in production-like environments
