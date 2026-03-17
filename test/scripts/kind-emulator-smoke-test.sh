#!/usr/bin/env bash
#
# Smoke test for the kind-emulator deployment.
# Assumes the cluster is already up with all pods running.
# See docs/kind-emulator-test-guide.md for setup instructions.
#

set -euo pipefail

# Configuration
LLMD_NS="${LLMD_NS:-llm-d-sim}"
AP_NS="${AP_NS:-async-processor-system}"
REDIS_NS="${REDIS_NS:-redis}"
REDIS_PORT="${REDIS_PORT:-6380}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"
MODEL="${MODEL:-unsloth/Meta-Llama-3.1-8B}"
DECODE_DEPLOYMENT="${DECODE_DEPLOYMENT:-ms-sim-llm-d-modelservice-decode}"
DECODE_REPLICAS="${DECODE_REPLICAS:-3}"

# Timeouts (seconds)
POLL_INTERVAL=2
METRIC_TIMEOUT=30
RESULT_TIMEOUT=60

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

cleanup_port_forwards() {
    info "Cleaning up port-forwards..."
    kill "$GW_PF_PID" 2>/dev/null || true
    kill "$REDIS_PF_PID" 2>/dev/null || true
}

# --- Pre-flight checks ---

info "Checking prerequisites..."

for cmd in kubectl redis-cli curl; do
    command -v "$cmd" &>/dev/null || fail "$cmd is required but not found"
done

info "Verifying pods in $LLMD_NS..."
kubectl wait --for=condition=Ready pod -l inferencepool=gaie-sim-epp -n "$LLMD_NS" --timeout=60s || \
    fail "EPP pod not ready"
kubectl wait --for=condition=Ready pod -l llm-d.ai/inferenceServing=true -n "$LLMD_NS" --timeout=60s || \
    fail "Model server pods not ready"

info "Verifying async-processor in $AP_NS..."
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=async-processor -n "$AP_NS" --timeout=60s 2>/dev/null || \
    kubectl get pods -n "$AP_NS" | grep -q "1/1.*Running" || \
    fail "Async processor pod not ready"

pass "All pods are running"

# --- Port-forwards ---

info "Setting up port-forwards..."
kubectl port-forward svc/infra-sim-inference-gateway-istio -n "$LLMD_NS" "$GATEWAY_PORT":80 &>/dev/null &
GW_PF_PID=$!

kubectl port-forward svc/redis-master -n "$REDIS_NS" "$REDIS_PORT":6379 &>/dev/null &
REDIS_PF_PID=$!

trap cleanup_port_forwards EXIT
sleep 2

redis-cli -p "$REDIS_PORT" ping | grep -q PONG || fail "Cannot connect to Redis"
pass "Port-forwards established"

# --- Test 1: Bootstrap saturation metric ---

info "Test 1: Bootstrapping saturation metric with direct HTTP request..."

RESPONSE=$(curl -s "http://localhost:$GATEWAY_PORT/v1/completions" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$MODEL\",\"prompt\":\"Say hello\",\"max_tokens\":10}")

echo "$RESPONSE" | grep -q '"choices"' || fail "Bootstrap request failed: $RESPONSE"
pass "Bootstrap request succeeded"

info "Waiting for Prometheus to scrape the metric..."
sleep 15

# --- Test 2: Async pipeline via Redis ---

info "Test 2: Testing async pipeline via Redis..."

redis-cli -p "$REDIS_PORT" DEL result-list &>/dev/null

DEADLINE=$(date -v+1H +%s 2>/dev/null || date -d '+1 hour' +%s)
redis-cli -p "$REDIS_PORT" ZADD request-sortedset "$DEADLINE" \
    "{\"id\":\"smoke-async-1\",\"deadline\":\"$DEADLINE\",\"payload\":{\"model\":\"$MODEL\",\"prompt\":\"Say hello\",\"max_tokens\":10},\"metadata\":{\"user\":\"smoke-test\"}}" >/dev/null

ELAPSED=0
while [ "$ELAPSED" -lt "$RESULT_TIMEOUT" ]; do
    RLEN=$(redis-cli -p "$REDIS_PORT" LLEN result-list)
    if [ "$RLEN" -ge 1 ]; then
        RESULT=$(redis-cli -p "$REDIS_PORT" LRANGE result-list 0 0)
        echo "$RESULT" | grep -q '"smoke-async-1"' || fail "Result ID mismatch"
        pass "Async pipeline: request processed successfully"
        break
    fi
    sleep "$POLL_INTERVAL"
    ELAPSED=$((ELAPSED + POLL_INTERVAL))
done

[ "$ELAPSED" -ge "$RESULT_TIMEOUT" ] && fail "Async pipeline: timed out waiting for result"

# --- Test 3: Saturation-based flow control ---

info "Test 3: Testing saturation-based flow control..."

redis-cli -p "$REDIS_PORT" DEL result-list &>/dev/null

# Scale down decode
info "Scaling $DECODE_DEPLOYMENT to 0..."
kubectl scale deployment "$DECODE_DEPLOYMENT" -n "$LLMD_NS" --replicas=0

info "Waiting for inference pods to terminate..."
ELAPSED=0
while [ "$ELAPSED" -lt "$METRIC_TIMEOUT" ]; do
    COUNT=$(kubectl get pods -n "$LLMD_NS" -l llm-d.ai/inferenceServing=true --no-headers 2>/dev/null | wc -l | tr -d ' ')
    [ "$COUNT" -eq 0 ] && break
    sleep "$POLL_INTERVAL"
    ELAPSED=$((ELAPSED + POLL_INTERVAL))
done
[ "$COUNT" -ne 0 ] && fail "Inference pods did not terminate"
pass "All inference pods terminated"

info "Waiting for saturation metric to reach 1..."
sleep 20

# Submit request (should be blocked)
DEADLINE=$(date -v+1H +%s 2>/dev/null || date -d '+1 hour' +%s)
redis-cli -p "$REDIS_PORT" ZADD request-sortedset "$DEADLINE" \
    "{\"id\":\"smoke-saturated-1\",\"deadline\":\"$DEADLINE\",\"payload\":{\"model\":\"$MODEL\",\"prompt\":\"Say hello\",\"max_tokens\":10},\"metadata\":{\"user\":\"smoke-test\"}}" >/dev/null

sleep 5
QLEN=$(redis-cli -p "$REDIS_PORT" ZCARD request-sortedset)
RLEN=$(redis-cli -p "$REDIS_PORT" LLEN result-list)

[ "$QLEN" -eq 1 ] || fail "Expected request queue length 1, got $QLEN"
[ "$RLEN" -eq 0 ] || fail "Expected result queue length 0, got $RLEN"
pass "Request is blocked (gate closed, saturation=1)"

# Scale back up
info "Scaling $DECODE_DEPLOYMENT back to $DECODE_REPLICAS..."
kubectl scale deployment "$DECODE_DEPLOYMENT" -n "$LLMD_NS" --replicas="$DECODE_REPLICAS"

info "Waiting for pods to become ready..."
kubectl wait --for=condition=Ready pod -l llm-d.ai/inferenceServing=true -n "$LLMD_NS" --timeout=120s || \
    fail "Model server pods did not become ready"

info "Waiting for request to be processed..."
ELAPSED=0
while [ "$ELAPSED" -lt "$RESULT_TIMEOUT" ]; do
    RLEN=$(redis-cli -p "$REDIS_PORT" LLEN result-list)
    if [ "$RLEN" -ge 1 ]; then
        RESULT=$(redis-cli -p "$REDIS_PORT" LRANGE result-list 0 0)
        echo "$RESULT" | grep -q '"smoke-saturated-1"' || fail "Result ID mismatch"
        QLEN=$(redis-cli -p "$REDIS_PORT" ZCARD request-sortedset)
        [ "$QLEN" -eq 0 ] || fail "Expected request queue to be empty, got $QLEN"
        pass "Request unblocked and processed after scale-up (saturation=0)"
        break
    fi
    sleep "$POLL_INTERVAL"
    ELAPSED=$((ELAPSED + POLL_INTERVAL))
done

[ "$ELAPSED" -ge "$RESULT_TIMEOUT" ] && fail "Timed out waiting for result after scale-up"

# --- Summary ---

echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  All smoke tests passed!${NC}"
echo -e "${GREEN}============================================${NC}"
