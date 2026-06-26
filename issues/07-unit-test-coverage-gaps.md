### Problem Statement

Several packages have source files with no corresponding test file:

| Package | Source files | Test files | Gap |
|---------|-------------|------------|-----|
| `cmd/` | 1 (`main.go`) | 0 | Flag parsing, TLS config builder, wiring logic |
| `internal/logging/` | 1 | 0 | Logger initialization |
| `pipeline/` | 1 | 0 | Interface definitions (testable via consumers but no direct tests) |
| `pkg/metrics/` | 1 | 0 | Metric registration |
| `pkg/util/` | 1 | 0 | Utility functions |
| `pkg/version/` | 1 | 0 | Version reporting |

Additionally, some tested packages have partial coverage:
- `pkg/redis/` has 5 source files but only 3 test files.
- `pkg/asyncworker/` has `http_client.go` + `worker.go` but only `worker_test.go`.

### Proposed Solution

1. Add unit tests for `cmd/main.go:buildTLSConfig` (table-driven: valid CA, mTLS, mismatched cert/key, insecure mode).
2. Add unit tests for `pkg/asyncworker/http_client.go`.
3. Add unit tests for `pkg/metrics/` metric registration.
4. Consider adding a CI coverage threshold (e.g. 60% minimum, increasing over time).

### Alternatives Considered

- Relying on integration and E2E tests to cover these paths — but unit tests provide faster feedback and isolate failures better.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- The CI runs `go test` with `-coverprofile` but there is no coverage threshold enforced.
- batch-gateway has unit tests for every internal package with substantive logic.
- The `buildTLSConfig` function in `cmd/main.go` has multiple code paths (CA only, mTLS, mismatched cert/key, insecure mode) that are completely untested.
- Graduation criteria addressed: Test coverage, Clear API/interface contracts (verified by tests).
