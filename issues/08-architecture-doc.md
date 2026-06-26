# Add architecture documentation

## Problem

There is no standalone architecture document for the async-processor. The README serves as both user guide and architecture overview, making it over 600 lines long. New contributors or evaluators have no quick way to understand:

- The internal component graph (main → Flow → Workers → InferenceClient → ResultChannel).
- How the `pipeline.Flow` abstraction maps to concrete implementations (Redis pubsub, Redis sorted-set, GCP Pub/Sub).
- The gate evaluation lifecycle (GateFactory → DispatchGate/AttributeGate → Budget check → Acquire).
- The request lifecycle (queue → merge policy → worker → dispatch → retry/result).
- The multi-module structure (`api/`, `pipeline/`, `producer/` as separate Go modules) and why.

## Context

- llm-d-inference-scheduler has `docs/architecture.md` covering plugin framework, request flow, and discovery.
- llm-d-kv-cache has `docs/architecture.md` covering indexing, scoring, and block lifecycle.
- batch-gateway has `docs/design/` with 6 architecture documents covering different subsystems.
- The llm-d well-lit-path doc for async processing links to `docs/architecture/advanced/batch/async-processor.md` which describes the external view but not the internal design.

## Proposed work

1. Create `docs/architecture.md` covering:
   - Component diagram (queue backends, gate subsystem, worker pool, inference client).
   - Request lifecycle (publish → consume → validate → gate check → dispatch → retry/result).
   - Gate evaluation model (budget gates vs attribute gates, composite gate composition).
   - Multi-module structure rationale.
2. Move the existing architecture diagrams from `docs/images/` into the doc with proper context.
3. Keep the README focused on user-facing quick-start and configuration reference.

## Graduation criteria addressed

- Documentation (architecture)
- Clear API/interface contracts
