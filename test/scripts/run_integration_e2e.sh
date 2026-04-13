#!/bin/bash

set -euo pipefail

cleanup() {
    echo "Interrupted! Cleaning up kind cluster..."
    kind delete cluster --name e2e-integration-tests 2>/dev/null || true
    exit 130
}

trap cleanup INT TERM

echo "Running integration end-to-end tests"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Default GAIE_ROOT to the sibling checkout if not set.
: "${GAIE_ROOT:=$(cd "$DIR/../../.." && pwd)/../kubernetes-sigs/gateway-api-inference-extension}"

export GAIE_ROOT

go test -v "${DIR}/../e2e/integration/" -ginkgo.v
