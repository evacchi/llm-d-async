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
func NewPodMetrics(prometheusURL string, poolName string) (*PodMetrics, error) {
	// Create PrometheusClient - required for pool saturation queries
	if prometheusURL == "" {
		setupLog.Error(nil, "PrometheusURL is required for pool saturation queries")
		return nil, nil
	}

	prometheusClient, err := NewPrometheusClient(prometheusURL)
	if err != nil {
		setupLog.Error(err, "Failed to create Prometheus client")
		return nil, err
	}
	setupLog.Info("Prometheus client configured for pool saturation queries",
		"prometheusURL", prometheusURL,
		"poolName", poolName)

	return &PodMetrics{
		prometheusClient: prometheusClient,
		poolName:         poolName,
	}, nil
}
