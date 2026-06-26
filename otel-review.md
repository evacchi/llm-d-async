# OTel Support Code Review

**Branch:** `otel` (vs `main`)
**Date:** 2026-06-01
**Scope:** 16 files, +1252 / -76 lines

## Findings

### 1. Helm: Redis tracing flag independent of OTel endpoint

**File:** `charts/async-processor/templates/ap-deployments.yaml:79`
**Severity:** Medium

The `--redis-tracing=true` flag is guarded by `.Values.ap.otel.redisTracing` independently of `.Values.ap.otel.endpoint`. A user can enable Redis tracing without configuring a collector endpoint.

**Failure scenario:** User sets `ap.otel.redisTracing=true` but leaves `ap.otel.endpoint=""`. redisotel instruments every Redis command creating spans, but `InitTracer` returns a no-op (no exporter configured). All Redis spans are silently dropped with only an INFO log at startup. User debugs for hours wondering why Redis traces don't appear.

### 2. DetachedContext bgCtx discarded in worker

**File:** `pkg/asyncworker/worker.go:105`
**Severity:** Low

`DetachedContext` creates a background context carrying a linked span and logger, but the returned context is immediately discarded with `_ = bgCtx`. The re-enqueue channel send doesn't use it.

**Failure scenario:** The span itself is created and ended correctly, so tracing works. However, the pattern signals "context not needed" when it's the designed carrier for the detached trace. A future maintainer adding traced operations to this block (e.g. logging with trace correlation) will need to un-discard bgCtx.

### 3. Propagator set to TraceContext only

**File:** `internal/otel/otel.go:100`
**Severity:** Low

`otel.SetTextMapPropagator(propagation.TraceContext{})` sets only W3C Trace Context propagation, excluding Baggage. batch-gateway uses a composite propagator.

**Failure scenario:** If a producer attaches OTel baggage entries (e.g. `tenant.id`, `priority`) to the request metadata alongside `traceparent`, `Extract()` will parse traceparent but silently ignore baggage headers. Downstream spans lose metadata that was meant to flow through the async pipeline.

## Comparison with batch-gateway OTel

### Shared patterns (good alignment)

- Same `InitTracer` shape: OTLP gRPC exporter, opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT`, no-op when unset
- Same `DetachedContext` pattern for shutdown re-enqueue (linked spans, background context)
- Same centralized attribute constants (`AttrRequestID`, `AttrQueueName`, etc.)
- Same otelhttp wrapping of the outbound HTTP client
- Same redisotel opt-in with a `redisTracing` flag
- Same Helm values structure (`endpoint`, `insecure`, `sampler`, `samplerArg`, `redisTracing`)
- Same 5-second graceful shutdown flush

### Gaps vs. batch-gateway

| Capability | batch-gateway | llm-d-async |
|---|---|---|
| HTTP middleware spans (per-route names) | Yes | No (only worker-level spans) |
| Storage-layer tracing | Yes (file ops) | N/A (no storage layer) |
| Cross-component context via DB tags | Yes (`otel:` prefix in DB) | Uses request metadata `traceparent` |
| PostgreSQL tracing | Yes (otelpgx, opt-in) | N/A (no PostgreSQL) |
| Baggage propagation | Yes (composite propagator) | No (TraceContext only) |

The gaps in middleware/storage tracing are expected given the architectural differences (llm-d-async is a simpler worker, not an API server). The baggage propagation gap is the one divergence worth considering aligning, especially if producers in the broader system attach baggage.
