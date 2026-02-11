package api

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/llm-d-incubation/llm-d-async/pkg/async/plugin"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/controller"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/datalayer"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/datastore"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/flowcontrol/contracts"
	fwkdl "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/datalayer"
	dlextractormetrics "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/plugins/datalayer/extractor/metrics"
	dlsourcemetrics "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/plugins/datalayer/source/metrics"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/requestcontrol"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/saturationdetector/framework/plugins/utilizationdetector"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

type PodMetrics struct {
	saturationDetector contracts.SaturationDetector
	podLocator         contracts.PodLocator
}

// NewPodMetrics creates the metrics collection subsystem for async-processor.
func NewPodMetrics(ctx context.Context, mgr manager.Manager, opts *MetricsOptions) *PodMetrics {
	// Create EndpointPool for pod discovery
	endpointPool := datalayer.NewEndpointPool(opts.TargetNamespace, opts.TargetPoolName)
	labelsMap, err := labels.ConvertSelectorToLabelsMap(opts.TargetLabelSelector)
	if err != nil {
		setupLog.Error(err, "Failed to parse label selector")
		return nil
	}
	endpointPool.Selector = labelsMap
	endpointPool.TargetPorts = opts.TargetPorts

	// Create plugin handle (will be populated later with datastore)
	handle := plugin.NewAsyncHandle(ctx, nil)

	// Configure and instantiate metrics data source plugin
	dataSourceParams := map[string]interface{}{
		"scheme":             opts.ModelServerMetricsScheme,
		"path":               opts.ModelServerMetricsPath,
		"insecureSkipVerify": opts.ModelServerMetricsHTTPSInsecure,
	}
	dataSourceParamsJSON, _ := json.Marshal(dataSourceParams)
	dataSourcePlugin, err := dlsourcemetrics.MetricsDataSourceFactory("metrics-source", dataSourceParamsJSON, handle)
	if err != nil {
		setupLog.Error(err, "Failed to create metrics data source plugin")
		return nil
	}

	// Configure and instantiate model server extractor plugin
	extractorParams := map[string]interface{}{
		"engineLabelKey": opts.EngineLabelKey,
		"defaultEngine":  opts.DefaultEngine,
	}
	extractorParamsJSON, _ := json.Marshal(extractorParams)
	extractorPlugin, err := dlextractormetrics.ModelServerExtractorFactory("model-server-extractor", extractorParamsJSON, handle)
	if err != nil {
		setupLog.Error(err, "Failed to create model server extractor plugin")
		return nil
	}

	// Build data layer config
	dataLayerConfig := &datalayer.Config{
		Sources: []datalayer.DataSourceConfig{
			{
				Plugin:     dataSourcePlugin.(fwkdl.DataSource),
				Extractors: []fwkdl.Extractor{extractorPlugin.(fwkdl.Extractor)},
			},
		},
	}

	// Create endpoint factory with pluggable data layer
	epFactory := datalayer.NewEndpointFactory(nil, opts.RefreshMetricsInterval)

	// Initialize data layer with plugin config
	const disallowedMetricsExtractor = ""
	if err := datalayer.WithConfig(dataLayerConfig, disallowedMetricsExtractor); err != nil {
		setupLog.Error(err, "Failed to configure data layer")
	}

	// Set data sources on the endpoint factory
	sources := datalayer.GetSources()
	epFactory.SetSources(sources)
	for _, src := range sources {
		setupLog.Info("Data layer configured", "source", src.TypedName().String(), "extractors", src.Extractors())
	}

	// Create datastore (modelServerMetricsPort parameter is deprecated, using EndpointPool.TargetPorts instead)
	ds := datastore.NewDatastore(ctx, epFactory, 0, datastore.WithEndpointPool(endpointPool))

	// Register PodReconciler to watch pods and populate the datastore
	podReconciler := &controller.PodReconciler{
		Datastore: ds,
		Reader:    mgr.GetClient(),
	}
	if err := podReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to setup PodReconciler with manager")
	}

	// Create PodLocator
	podLocator := requestcontrol.NewDatastorePodLocator(ds, requestcontrol.WithDisableEndpointSubsetFilter(false))

	// Build saturation detector config
	saturationDetectorConfig := &utilizationdetector.Config{
		QueueDepthThreshold:       opts.QueueDepthThreshold,
		KVCacheUtilThreshold:      opts.KVCacheUtilThreshold,
		MetricsStalenessThreshold: opts.MetricsStalenessThreshold,
	}

	// Create SaturationDetector using gateway's utilization detector
	saturationDetector := utilizationdetector.NewDetector(saturationDetectorConfig, setupLog)

	return &PodMetrics{
		podLocator:         podLocator,
		saturationDetector: saturationDetector,
	}
}
