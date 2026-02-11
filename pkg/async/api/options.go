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
	"fmt"
	"time"

	"github.com/spf13/pflag"
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

// AddFlags adds flags for MetricsOptions to the specified FlagSet.
func (opts *MetricsOptions) AddFlags(fs *pflag.FlagSet) {
	if fs == nil {
		fs = pflag.CommandLine
	}

	// Data layer flags
	fs.StringVar(&opts.ModelServerMetricsScheme, "model-server-metrics-scheme", opts.ModelServerMetricsScheme,
		"Protocol scheme used in scraping metrics from endpoints (http or https)")
	fs.StringVar(&opts.ModelServerMetricsPath, "model-server-metrics-path", opts.ModelServerMetricsPath,
		"URL path used in scraping metrics from endpoints")
	fs.BoolVar(&opts.ModelServerMetricsHTTPSInsecure, "model-server-metrics-https-insecure-skip-verify", opts.ModelServerMetricsHTTPSInsecure,
		"Skip TLS certificate verification when using 'https' scheme")
	fs.DurationVar(&opts.RefreshMetricsInterval, "refresh-metrics-interval", opts.RefreshMetricsInterval,
		"Interval to refresh metrics from model servers")
	fs.DurationVar(&opts.MetricsStalenessThreshold, "metrics-staleness-threshold", opts.MetricsStalenessThreshold,
		"Duration after which metrics are considered stale")

	// Extractor flags
	fs.StringVar(&opts.EngineLabelKey, "engine-label-key", opts.EngineLabelKey,
		"Pod label key for identifying the engine type (vllm, sglang, etc.)")
	fs.StringVar(&opts.DefaultEngine, "default-engine", opts.DefaultEngine,
		"Default engine type to use when pod label is missing")

	// Pod discovery flags
	fs.StringVar(&opts.TargetNamespace, "target-namespace", opts.TargetNamespace,
		"Namespace to watch for model server pods")
	fs.StringVar(&opts.TargetPoolName, "target-pool-name", opts.TargetPoolName,
		"Name of the endpoint pool")
	fs.StringVar(&opts.TargetLabelSelector, "target-label-selector", opts.TargetLabelSelector,
		"Label selector for discovering model server pods")
	fs.IntSliceVar(&opts.TargetPorts, "target-ports", opts.TargetPorts,
		"Target ports to scrape metrics from")

	// Saturation detector flags
	fs.IntVar(&opts.QueueDepthThreshold, "queue-depth-threshold", opts.QueueDepthThreshold,
		"Queue depth above which a pod is considered saturated")
	fs.Float64Var(&opts.KVCacheUtilThreshold, "kv-cache-util-threshold", opts.KVCacheUtilThreshold,
		"KV cache utilization (0.0 to 1.0) above which a pod is considered saturated")
}

// Complete performs any post-processing on the options.
func (opts *MetricsOptions) Complete() error {
	// No post-processing needed currently
	return nil
}

// Validate validates the options.
func (opts *MetricsOptions) Validate() error {
	if opts.ModelServerMetricsScheme != "http" && opts.ModelServerMetricsScheme != "https" {
		return fmt.Errorf("model-server-metrics-scheme must be 'http' or 'https', got %q", opts.ModelServerMetricsScheme)
	}

	if opts.RefreshMetricsInterval <= 0 {
		return fmt.Errorf("refresh-metrics-interval must be positive, got %v", opts.RefreshMetricsInterval)
	}

	if opts.MetricsStalenessThreshold <= 0 {
		return fmt.Errorf("metrics-staleness-threshold must be positive, got %v", opts.MetricsStalenessThreshold)
	}

	if opts.TargetNamespace == "" {
		return fmt.Errorf("target-namespace cannot be empty")
	}

	if opts.TargetLabelSelector == "" {
		return fmt.Errorf("target-label-selector cannot be empty")
	}

	if len(opts.TargetPorts) == 0 {
		return fmt.Errorf("target-ports cannot be empty")
	}

	for _, port := range opts.TargetPorts {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid port number %d in target-ports", port)
		}
	}

	if opts.QueueDepthThreshold <= 0 {
		return fmt.Errorf("queue-depth-threshold must be positive, got %d", opts.QueueDepthThreshold)
	}

	if opts.KVCacheUtilThreshold <= 0 || opts.KVCacheUtilThreshold > 1.0 {
		return fmt.Errorf("kv-cache-util-threshold must be between 0.0 and 1.0, got %f", opts.KVCacheUtilThreshold)
	}

	return nil
}
