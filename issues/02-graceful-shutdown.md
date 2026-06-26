### Problem Statement

`cmd/main.go` uses `ctrl.SetupSignalHandler()` for context cancellation on SIGTERM/SIGINT, and the worker goroutines exit when the context is done. However:

1. There is no mechanism to stop pulling new messages from the queue while draining in-flight requests. Workers compete with new pulls until the context is cancelled, at which point both new and in-flight work are abandoned simultaneously.
2. The OTel tracing E2E test (`e2e_otel_test.go`) documents a `re-enqueue` span for graceful shutdown, but the actual shutdown path in `main.go` is just `impl.Shutdown()` after the wait group completes — there is no two-phase shutdown (stop consuming → drain → shutdown).
3. For the Redis sorted-set implementation, messages popped from the sorted set during shutdown may be lost if the worker is interrupted before re-enqueuing them.

### Proposed Solution

1. Implement two-phase shutdown: on SIGTERM, stop accepting new messages from the queue, then drain in-flight requests up to a configurable timeout.
2. Re-enqueue any messages that were claimed but not completed before the drain timeout.
3. Add integration tests for the shutdown path (verify no message loss on SIGTERM).
4. Coordinate with the health probe work: readiness probe should go unhealthy during drain phase.

### Alternatives Considered

- Relying solely on the queue's at-least-once delivery semantics (messages time out and become visible again) — but this introduces unnecessary latency for retried messages and doesn't work for Redis pub/sub which is ephemeral.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- batch-gateway has explicit graceful shutdown testing (`test/e2e/processor_graceful_shutdown_test.go`) and a dedicated recovery/orphan-recovery subsystem.
- The chart sets `terminationGracePeriodSeconds: 130` which implies shutdown can take over 2 minutes, but the code does not use that budget for draining.
- Graduation criteria addressed: Reliability, Tested by users in production-like environments.
