package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"github.com/llm-d-incubation/llm-d-async/internal/logging"
	"github.com/llm-d-incubation/llm-d-async/pkg/async"
	"github.com/llm-d-incubation/llm-d-async/pkg/async/api"
	"github.com/llm-d-incubation/llm-d-async/pkg/metrics"
	"github.com/llm-d-incubation/llm-d-async/pkg/pubsub"
	"github.com/llm-d-incubation/llm-d-async/pkg/redis"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	fwkplugin "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
	dlextractormetrics "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/plugins/datalayer/extractor/metrics"
	dlsourcemetrics "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/plugins/datalayer/source/metrics"
)

func main() {

	var loggerVerbosity int

	var metricsPort int
	var metricsEndpointAuth bool

	var concurrency int
	var requestMergePolicy string
	var messageQueueImpl string

	var prometheusURL string
	var poolName string

	flag.IntVar(&loggerVerbosity, "v", logging.DEFAULT, "number for the log level verbosity")

	flag.IntVar(&metricsPort, "metrics-port", 9090, "The metrics port")
	flag.BoolVar(&metricsEndpointAuth, "metrics-endpoint-auth", true, "Enables authentication and authorization of the metrics endpoint")

	flag.IntVar(&concurrency, "concurrency", 8, "number of concurrent workers")

	flag.StringVar(&requestMergePolicy, "request-merge-policy", "random-robin", "The request merge policy to use. Supported policies: random-robin")
	flag.StringVar(&messageQueueImpl, "message-queue-impl", "redis-pubsub", "The message queue implementation to use. Supported implementations: redis-pubsub")

	// Prometheus configuration
	flag.StringVar(&prometheusURL, "prometheus-url", prometheusURL,
		"URL of Prometheus server for querying pool saturation (e.g., http://prometheus.monitoring.svc.cluster.local:9090)")
	flag.StringVar(&poolName, "pool-name", poolName,
		"Name of the inference pool to query saturation for")

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)

	logging.InitLogging(&opts, loggerVerbosity)
	defer logging.Sync() // nolint:errcheck

	setupLog := ctrl.Log.WithName("setup")
	setupLog.Info("Logger initialized")

	////////setupLog.Info("GIE build", "commit-sha", version.CommitSHA, "build-ref", version.BuildRef)

	printAllFlags(setupLog)

	metrics.Register(metrics.GetAsyncProcessorCollectors()...)

	// Register data layer plugins
	fwkplugin.Register(dlsourcemetrics.MetricsDataSourceType, dlsourcemetrics.MetricsDataSourceFactory)
	fwkplugin.Register(dlextractormetrics.MetricsExtractorType, dlextractormetrics.ModelServerExtractorFactory)

	ctx := ctrl.SetupSignalHandler()

	// Register metrics handler.
	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress: fmt.Sprintf(":%d", metricsPort),
		FilterProvider: func() func(c *rest.Config, httpClient *http.Client) (metricsserver.Filter, error) {
			if metricsEndpointAuth {
				return filters.WithAuthenticationAndAuthorization
			}

			return nil
		}(),
	}
	restConfig := ctrl.GetConfigOrDie()
	httpClient := http.DefaultClient

	msrv, _ := metricsserver.NewServer(metricsServerOptions, restConfig, httpClient)
	go msrv.Start(ctx) // nolint:errcheck

	/////

	var policy api.RequestMergePolicy
	switch requestMergePolicy {
	case "random-robin":
		policy = async.NewRandomRobinPolicy()
	default:
		setupLog.Error(nil, "Unknown request merge policy", "request-merge-policy", requestMergePolicy)
		os.Exit(1)
	}

	var impl api.Flow
	switch messageQueueImpl {
	case "redis-pubsub":
		impl = redis.NewRedisMQFlow()
	case "gcp-pubsub":
		impl = pubsub.NewGCPPubSubMQFlow()
	default:
		setupLog.Error(nil, "Unknown message queue implementation", "message-queue-impl", messageQueueImpl)
		os.Exit(1)
	}

	// Create a controller-runtime manager
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{})
	if err != nil {
		setupLog.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	// Create podMetrics with the manager so datastore reconcilers can be registered
	podMetrics := api.NewPodMetrics(prometheusURL, poolName)

	requestChannel := policy.MergeRequestChannels(impl.RequestChannels()).Channel
	for w := 1; w <= concurrency; w++ {
		go api.Worker(ctx, impl.Characteristics(), podMetrics, httpClient, requestChannel, impl.RetryChannel(), impl.ResultChannel())
	}

	impl.Start(ctx)

	// Start the manager (this starts all reconcilers and blocks)
	setupLog.Info("Starting controller manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}
}

func printAllFlags(setupLog logr.Logger) {
	flags := make(map[string]any)
	flag.VisitAll(func(f *flag.Flag) {
		flags[f.Name] = f.Value
	})
	setupLog.Info("Flags processed", "flags", flags)
}
