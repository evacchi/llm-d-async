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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/observability/logging"
)

// PrometheusClient queries Prometheus for metrics.
type PrometheusClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPrometheusClient creates a new Prometheus query client.
func NewPrometheusClient(baseURL string) *PrometheusClient {
	// Skip TLS verification for internal cluster communication
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &PrometheusClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

// prometheusResponse represents the Prometheus query API response.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// QueryPoolSaturation queries the inference_extension_flow_control_pool_saturation metric.
// Returns the saturation value (0.0-1.0) or an error.
func (pc *PrometheusClient) QueryPoolSaturation(ctx context.Context, poolName string) (float64, error) {
	logger := log.FromContext(ctx)

	// Build the PromQL query
	query := fmt.Sprintf(`inference_extension_flow_control_pool_saturation{inference_pool="%s"}`, poolName)

	// Build the query URL
	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", pc.baseURL, url.QueryEscape(query))

	logger.V(logutil.DEBUG).Info("Querying Prometheus for pool saturation",
		"url", queryURL,
		"poolName", poolName)

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return 0.0, fmt.Errorf("failed to create Prometheus request: %w", err)
	}

	// Execute the request
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return 0.0, fmt.Errorf("failed to query Prometheus: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0.0, fmt.Errorf("failed to read Prometheus response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return 0.0, fmt.Errorf("Prometheus query failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var promResp prometheusResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return 0.0, fmt.Errorf("failed to parse Prometheus response: %w", err)
	}

	// Check status
	if promResp.Status != "success" {
		return 0.0, fmt.Errorf("Prometheus query returned status: %s", promResp.Status)
	}

	// Extract the value
	if len(promResp.Data.Result) == 0 {
		// No data found - this could mean:
		// 1. The metric hasn't been scraped yet
		// 2. The pool name doesn't match
		// Return 0.0 and let the caller decide what to do
		logger.V(logutil.DEBUG).Info("No saturation metric found for pool",
			"poolName", poolName)
		return 0.0, nil
	}

	// Get the first result
	result := promResp.Data.Result[0]
	if len(result.Value) < 2 {
		return 0.0, fmt.Errorf("unexpected Prometheus value format")
	}

	// The value is [timestamp, "value_as_string"]
	valueStr, ok := result.Value[1].(string)
	if !ok {
		return 0.0, fmt.Errorf("unexpected value type in Prometheus response")
	}

	// Parse the saturation value
	var saturation float64
	if _, err := fmt.Sscanf(valueStr, "%f", &saturation); err != nil {
		return 0.0, fmt.Errorf("failed to parse saturation value: %w", err)
	}

	logger.V(logutil.DEBUG).Info("Queried pool saturation from Prometheus",
		"poolName", poolName,
		"saturation", saturation)

	return saturation, nil
}
