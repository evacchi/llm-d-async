# Complete and test the producer SDK

## Problem

The `producer/` submodule provides a `Producer` interface and a single implementation (`RedisSortedSetProducer`). This is the SDK for submitting requests to the async-processor. Gaps:

1. **Only Redis sorted-set is implemented.** There is no producer for Redis pub/sub or GCP Pub/Sub. Users of those backends must construct and publish messages manually.
2. **Limited test coverage.** `producer/redis_sortedset_producer_test.go` exists but there are no integration tests for the producer (e.g. round-trip: produce → consume → verify).
3. **No usage examples.** The README shows `redis-cli` commands for testing. There is no Go example showing how to use the producer SDK programmatically.
4. **No client library for non-Go consumers.** Python is a primary language for ML workloads; users working with batch inference from Python have no SDK.

## Context

- batch-gateway has a full REST API that clients interact with (API server with documented endpoints). The async-processor intentionally takes a different approach (BYOQ), but the producer SDK is the consumer-facing contract.
- The `api/` submodule defines `RequestMessage` which producers must serialize correctly (including the internal `InternalMessage` wire format with `request_kind` and `data` fields, as shown in the development section of the README).

## Proposed work

1. Add a Redis pub/sub producer implementation.
2. Add a GCP Pub/Sub producer implementation.
3. Add integration tests that verify produce → consume round-trip for each backend.
4. Add a Go usage example in `examples/` showing how to submit a batch of requests and consume results.
5. Consider a Python producer package (or at minimum, document the wire format clearly enough for any language to implement).

## Graduation criteria addressed

- Clear API/interface contracts
- Documentation (user-facing)
- For standalone: can be consumed without breaking existing flows
