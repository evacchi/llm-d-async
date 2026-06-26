# Improve integration story with llm-d well-lit path guides

## Problem

The llm-d well-lit path doc (`docs/well-lit-paths/asynchronous-processing.md`) and the guide (`guides/asynchronous-processing/`) exist, but the integration story has gaps:

1. **The well-lit path doc is high-level** — it describes the concept but delegates all specifics to the guide, which requires a manual multi-step process (install CRDs, create namespace, deploy model server, deploy EPP, deploy Redis, deploy async-processor, configure Prometheus relabeling).
2. **No end-to-end quick-start.** A user cannot go from zero to working async processing in under 10 minutes. The closest is `make deploy-ap-emulated-on-kind` but it requires a local checkout and specific env vars.
3. **The `dispatch-budget.md` doc** describes the algorithm but doesn't explain how to set the prerequisite metrics pipeline (EPP flow control plugin, vLLM metric relabeling).
4. **No integration test with the real llm-d stack in CI.** The E2E tests use `llm-d-inference-sim` (a mock), not the actual inference scheduler.

## Context

- Other well-lit paths (e.g. `optimized-baseline`, `pd-disaggregation`) have step-by-step guides with specific Helm values and verification commands.
- The `docs/guides/e2e-deploy/` directory has a detailed real-cluster guide but it's only in the async repo, not linked from the main llm-d guides.
- The llm-d `guides/asynchronous-processing/` directory has backend-specific subdirs (redis, gcp-pubsub) but the relationship to the main guide is not immediately clear.

## Proposed work

1. Create a streamlined quick-start guide that works on KIND with the emulated setup (< 10 minutes, single `make` command + verify step).
2. Cross-link the `docs/guides/e2e-deploy/` guide from the llm-d well-lit path doc.
3. Add a "prerequisites" section to the dispatch-budget doc explaining the metrics pipeline setup.
4. Consider contributing a PR to llm-d/llm-d to improve the asynchronous-processing guide with concrete verification steps.

## Graduation criteria addressed

- For well-lit path: clear integration story with existing guides
- Documentation (user-facing)
