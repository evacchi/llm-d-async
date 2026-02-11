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
	asyncserver "github.com/llm-d-incubation/llm-d-async/pkg/async/server"
	"github.com/llm-d-incubation/llm-d-async/pkg/metrics"
	"github.com/llm-d-incubation/llm-d-async/pkg/pubsub"
	"github.com/llm-d-incubation/llm-d-async/pkg/redis"
	"github.com/spf13/pflag"
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

	flag.IntVar(&loggerVerbosity, "v", logging.DEFAULT, "number for the log level verbosity")

	flag.IntVar(&metricsPort, "metrics-port", 9090, "The metrics port")
	flag.BoolVar(&metricsEndpointAuth, "metrics-endpoint-auth", true, "Enables authentication and authorization of the metrics endpoint")

	flag.IntVar(&concurrency, "concurrency", 8, "number of concurrent workers")

	flag.StringVar(&requestMergePolicy, "request-merge-policy", "random-robin", "The request merge policy to use. Supported policies: random-robin")
	flag.StringVar(&messageQueueImpl, "message-queue-impl", "redis-pubsub", "The message queue implementation to use. Supported implementations: redis-pubsub")

	// Create metrics options and register flags
	metricsOpts := api.NewMetricsOptions()
	metricsOpts.AddFlags(pflag.CommandLine)

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)

	// Add standard flags to pflag command line
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// Complete and validate metrics options
	if err := metricsOpts.Complete(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to complete metrics options: %v\n", err)
		os.Exit(1)
	}
	if err := metricsOpts.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid metrics options: %v\n", err)
		os.Exit(1)
	}

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

	mgr, err := asyncserver.NewDefaultManager(metricsOpts.TargetNamespace, restConfig, metricsServerOptions)
	if err != nil {
		setupLog.Error(err, "Failed to create manager")
		os.Exit(1)
	}

	// Create podMetrics with the manager so datastore reconcilers can be registered
	podMetrics := api.NewPodMetrics(ctx, mgr, metricsOpts)

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
