/*
Copyright 2026 The llm-d-incubation Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/observability/logging"
)

// PrometheusClient queries Prometheus for metrics.
type PrometheusClient struct {
	api v1.API
}

// NewPrometheusClient creates a new Prometheus query client using the official Prometheus API client.
func NewPrometheusClient(baseURL string) (*PrometheusClient, error) {
	client, err := api.NewClient(api.Config{
		Address: baseURL,
		RoundTripper: &http.Transport{
			// FIXME: Skip TLS verification for internal cluster communication
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus API client: %w", err)
	}

	return &PrometheusClient{
		api: v1.NewAPI(client),
	}, nil
}

// QueryPoolSaturation queries the inference_extension_flow_control_pool_saturation metric.
// Returns the saturation value (0.0-1.0) or an error.
func (pc *PrometheusClient) QueryPoolSaturation(ctx context.Context, poolName string) (float64, error) {
	logger := log.FromContext(ctx)

	// Build the PromQL query
	query := fmt.Sprintf(`inference_extension_flow_control_pool_saturation{inference_pool="%s"}`, poolName)

	logger.V(logutil.DEBUG).Info("Querying Prometheus for pool saturation",
		"query", query,
		"poolName", poolName)

	// Execute the query using the Prometheus API client
	result, warnings, err := pc.api.Query(ctx, query, time.Now())
	if err != nil {
		return 0.0, fmt.Errorf("failed to query Prometheus: %w", err)
	}

	// Log any warnings
	if len(warnings) > 0 {
		logger.V(logutil.DEFAULT).Info("Prometheus query returned warnings",
			"warnings", warnings)
	}

	// Check if we got results
	if result.Type() != model.ValVector {
		return 0.0, fmt.Errorf("unexpected result type: %s", result.Type())
	}

	vector := result.(model.Vector)
	if len(vector) == 0 {
		// No data found - this could mean:
		// 1. The metric hasn't been scraped yet
		// 2. The pool name doesn't match
		// Return 0.0 and let the caller decide what to do
		logger.V(logutil.DEBUG).Info("No saturation metric found for pool",
			"poolName", poolName)
		return 0.0, nil
	}

	// Get the first result
	sample := vector[0]
	saturation := float64(sample.Value)

	logger.V(logutil.DEBUG).Info("Queried pool saturation from Prometheus",
		"poolName", poolName,
		"saturation", saturation,
		"timestamp", sample.Timestamp)

	return saturation, nil
}
