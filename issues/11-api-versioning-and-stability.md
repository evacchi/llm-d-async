# Define API stability guarantees and versioning policy

## Problem

The `api/` and `pipeline/` Go submodules define the public contract consumed by producers and alternative queue backends. These are currently at `v0.7.0-rc.4` but there is no documented stability guarantee:

- What constitutes a breaking change? (adding a method to the `Request` interface is breaking for implementers)
- What is the compatibility promise between major/minor versions?
- How do consumers know when the API is stable enough for production use?

The `producer/` submodule also depends on `api/` — cross-module version coordination is handled by `make set-version` but there is no documented policy for when to bump major vs minor.

## Context

- The multi-module structure (`api/`, `pipeline/`, `producer/` each with their own `go.mod`) is designed for composability — third parties can depend on `api/` without pulling the full processor. This is good, but it raises the bar for API stability communication.
- Graduated llm-d projects have documented API references (e.g. `docs/api-reference/` in llm-d/llm-d).
- The `Request` interface has 7 methods; adding an 8th would break all external implementers.

## Proposed work

1. Document the versioning policy in `CONTRIBUTING.md` or a dedicated `docs/api-stability.md`:
   - What the Go module versions mean (alpha → beta → stable).
   - Breaking change policy for the `api/` and `pipeline/` interfaces.
   - Deprecation process.
2. Consider whether the `Request` and `Flow` interfaces should be narrowed or frozen before v1.0.
3. Add a compatibility test that verifies the public API surface has not changed unexpectedly (e.g. using `apidiff` or similar tooling).

## Graduation criteria addressed

- Clear API/interface contracts
- For standalone: can be consumed without breaking existing flows
