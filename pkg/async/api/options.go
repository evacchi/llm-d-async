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
	"time"
)

// MetricsOptions contains configuration for async-processor metrics collection.
type MetricsOptions struct {
	//
	// MSP metrics scraping.
	//
	ModelServerMetricsScheme         string        // Protocol scheme used in scraping metrics from endpoints.
	ModelServerMetricsPath           string        // URL path used in scraping metrics from endpoints.
	ModelServerMetricsHTTPSInsecure  bool          // Disable certificate verification when using 'https' scheme for 'model-server-metrics-scheme'.
	RefreshMetricsInterval           time.Duration // Interval to refresh metrics.
	RefreshPrometheusMetricsInterval time.Duration // Interval to flush Prometheus metrics.
	MetricsStalenessThreshold        time.Duration // Duration after which metrics are considered stale.
	TotalQueuedRequestsMetric        string        // Prometheus metric specification for the number of queued requests.
	TotalRunningRequestsMetric       string        // Prometheus metric specification for the number of running requests.
	KVCacheUsagePercentageMetric     string        // Prometheus metric specification for the fraction of KV-cache blocks currently in use.
	LoRAInfoMetric                   string        // Prometheus metric specification for the LoRA info metrics.
	CacheInfoMetric                  string        // Prometheus metric specification for the cache info metrics.

	// Model server extractor configuration
	EngineLabelKey string // Pod label key for engine type
	DefaultEngine  string // Default engine type when label is missing

	// Pod discovery
	TargetNamespace     string // Namespace to watch for model server pods
	TargetPoolName      string // Name of the endpoint pool
	TargetLabelSelector string // Label selector for discovering model server pods
	TargetPorts         []int  // Target ports to scrape metrics from

	// Saturation detector thresholds
	QueueDepthThreshold  int     // Queue depth above which a pod is saturated
	KVCacheUtilThreshold float64 // KV cache utilization (0.0 to 1.0) above which a pod is saturated
}

// NewMetricsOptions returns a new MetricsOptions struct initialized with default values.
func NewMetricsOptions() *MetricsOptions {
	return &MetricsOptions{
		// Data layer defaults
		ModelServerMetricsScheme:        "http",
		ModelServerMetricsPath:          "/metrics",
		ModelServerMetricsHTTPSInsecure: false,
		RefreshMetricsInterval:          5 * time.Second,
		MetricsStalenessThreshold:       30 * time.Second,

		// Extractor defaults
		EngineLabelKey: "inference.networking.k8s.io/engine-type",
		DefaultEngine:  "vllm",

		// Pod discovery defaults
		TargetNamespace:     "llm-d-sim",
		TargetPoolName:      "ms-sim-llm-d-modelservice-decode",
		TargetLabelSelector: "llm-d.ai/inferenceServing=true,llm-d.ai/role=decode",
		TargetPorts:         []int{8000},

		// Saturation detector defaults
		QueueDepthThreshold:  10,
		KVCacheUtilThreshold: 0.9,
	}
}
