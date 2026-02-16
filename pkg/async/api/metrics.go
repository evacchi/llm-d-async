package api

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

type PodMetrics struct {
	prometheusClient *PrometheusClient
	poolName         string
}

// NewPodMetrics creates the metrics collection subsystem for async-processor.
func NewPodMetrics(prometheusURL string, poolName string) *PodMetrics {
	// Create PrometheusClient - required for pool saturation queries
	if prometheusURL == "" {
		setupLog.Error(nil, "PrometheusURL is required for pool saturation queries")
		return nil
	}

	prometheusClient := NewPrometheusClient(prometheusURL)
	setupLog.Info("Prometheus client configured for pool saturation queries",
		"prometheusURL", prometheusURL,
		"poolName", poolName)

	return &PodMetrics{
		prometheusClient: prometheusClient,
		poolName:         poolName,
	}
}
