# Kind Emulator: Setup and End-to-End Test Guide

## Prerequisites

- `kind`, `kubectl`, `helm`, `helmfile`, `docker` installed
- `gateway-api-inference-extension:dev` image built locally
- `redis-cli` installed (for testing)

## 1. Create the Kind Cluster

Remove any leftover `llm-d` clone from previous runs, then deploy:

```bash
rm -rf ./llm-d

make deploy-ap-emulated-on-kind \
  DISPATCH_GATE_TYPE=metric-saturation \
  GAIE_IMAGE=gateway-api-inference-extension:dev \
  REDIS_IMPL=redis-sortedset-gated \
  DEPLOY_LLM_D=true \
  DEPLOY_PROMETHEUS=true \
  DEPLOY_REDIS=true \
  CREATE_CLUSTER=true
```

## 2. Load the GAIE Image into the Kind Cluster

The custom EPP image must be loaded into the kind cluster so that nodes can
pull it locally:

```bash
kind load docker-image gateway-api-inference-extension:dev --name kind-ap-gpu-cluster
```

## 3. Verify All Pods Are Running

```bash
kubectl get pods -n llm-d-sim
kubectl get pods -n async-processor-system
```

All pods should be in `Running` state with all containers ready.

## 4. Bootstrap the Saturation Metric

The `metric-saturation` dispatch gate requires the
`inference_extension_flow_control_pool_saturation` metric to exist in Prometheus.
This metric is only emitted by the EPP after it processes at least one request.
Without it, the gate fails closed and blocks all async processing.

Send a direct request to the inference gateway to bootstrap the metric:

```bash
kubectl port-forward svc/infra-sim-inference-gateway-istio -n llm-d-sim 8080:80 &

curl -s http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"unsloth/Meta-Llama-3.1-8B","prompt":"Say hello","max_tokens":10}'
```

Wait ~15 seconds for Prometheus to scrape the new metric.

## 5. Test the Async Pipeline via Redis

Port-forward Redis and submit a request to the sorted-set request queue:

```bash
kubectl port-forward svc/redis-master -n redis 6380:6379 &
sleep 2

DEADLINE=$(date -v+1H +%s)  # macOS; use date -d '+1 hour' +%s on Linux
redis-cli -p 6380 ZADD request-sortedset $DEADLINE \
  '{"id":"test-req-1","deadline":"'"$DEADLINE"'","payload":{"model":"unsloth/Meta-Llama-3.1-8B","prompt":"Say hello","max_tokens":10},"metadata":{"user":"manual-test"}}'
```

Wait for processing, then read the result:

```bash
sleep 5
redis-cli -p 6380 LRANGE result-list 0 -1
```

You should see a JSON result with `id: test-req-1` and a completion payload.

## 6. Test Saturation-Based Flow Control

This test verifies that the dispatch gate blocks requests when model servers are
unavailable (saturation=1) and releases them once servers are back (saturation=0).

### Scale down the model servers

Scale the decode deployment to zero replicas (prefill is disabled in this setup):

```bash
kubectl scale deployment ms-sim-llm-d-modelservice-decode -n llm-d-sim --replicas=0
```

Wait ~20 seconds for the saturation metric to reach `1`:

```bash
kubectl get pods -n llm-d-sim -l llm-d.ai/inferenceServing=true  # should return no resources
```

### Submit a request (it should be blocked)

```bash
DEADLINE=$(date -v+1H +%s)  # macOS; use date -d '+1 hour' +%s on Linux
redis-cli -p 6380 ZADD request-sortedset $DEADLINE \
  '{"id":"test-saturated-1","deadline":"'"$DEADLINE"'","payload":{"model":"unsloth/Meta-Llama-3.1-8B","prompt":"Say hello","max_tokens":10},"metadata":{"user":"manual-test"}}'
```

Verify the request is stuck (not consumed, no result):

```bash
sleep 5
redis-cli -p 6380 ZCARD request-sortedset  # should be 1
redis-cli -p 6380 LLEN result-list          # should be 0
```

### Scale model servers back up

```bash
kubectl scale deployment ms-sim-llm-d-modelservice-decode -n llm-d-sim --replicas=3
```

Wait for pods to become ready and saturation to drop (~30-40 seconds), then check the result:

```bash
redis-cli -p 6380 ZCARD request-sortedset  # should be 0
redis-cli -p 6380 LRANGE result-list 0 -1  # should contain the result for test-saturated-1
```

## 7. Teardown

```bash
make undeploy-ap-emulated-on-kind \
  DELETE_CLUSTER=true \
  DEPLOY_LLM_D=true \
  DEPLOY_REDIS=true \
  DEPLOY_PROMETHEUS=true
```
