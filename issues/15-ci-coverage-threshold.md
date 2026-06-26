# Enforce test coverage threshold in CI

## Problem

The `make test` target generates `cover.out` and the producer submodule generates `cover-producer.out`, but no CI step checks the coverage percentage or fails on regression. There is no visibility into current coverage or trends.

## Context

- The CI runs `make test` and `make test-integration` but does not upload or check coverage results.
- llm-d-inference-scheduler CI (`ci-pr-checks.yaml`) includes coverage reporting.
- Current coverage is unknown — the `cover.out` file exists in the repo (committed, likely accidentally) but is not gated.

## Proposed work

1. Add a CI step that parses `cover.out` and reports the coverage percentage.
2. Set an initial coverage floor (e.g. 50% based on current state, to be raised over time).
3. Fail the CI if coverage drops below the floor.
4. Remove the committed `cover.out` file from the repo (add to `.gitignore`).
5. Optionally integrate with a coverage service (Codecov, Coveralls) for PR-level diff coverage.

## Graduation criteria addressed

- Test coverage
- CI/CD
