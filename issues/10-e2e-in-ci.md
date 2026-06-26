### Problem Statement

The Makefile defines `test-e2e` (Ginkgo suite against a KIND cluster) and `test-integration` (spawns mock servers). The CI workflow (`ci-pr-checks.yaml`) runs `make test` (unit) and `make test-integration`, but does **not** run `make test-e2e`.

The E2E test suite is substantial (12 test files covering core flow, saturation gates, budget gates, query gates, attribute gates, composite gates, endpoint scrape gates, OTel tracing), but it only runs when a developer explicitly runs it locally. Regressions in the E2E path are not caught before merge.

### Proposed Solution

1. Add a CI workflow (e.g. `ci-e2e-tests.yaml`) that:
   - Spins up a KIND cluster using the existing `deploy/kind-emulator/setup.sh`
   - Builds the async-processor image
   - Runs `make test-e2e`
   - Tears down the cluster (or skips cleanup on failure for artifact collection)
2. Consider running E2E on a schedule (nightly) if the full suite is too slow for every PR, with a lighter smoke subset on PR.
3. Add E2E test results to required status checks.

### Alternatives Considered

- Running E2E only on release branches — but this delays regression detection and makes bisecting harder.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- batch-gateway runs E2E integration tests in CI (`ci-integration-tests.yml`) as a separate workflow.
- llm-d-inference-scheduler runs E2E tests in CI (`ci-dev.yaml`).
- The E2E suite already supports `E2E_SKIP_CLEANUP` and configurable NodePort overrides for CI use.
- Graduation criteria addressed: Test coverage, Tested by users in production-like environments, CI/CD.
