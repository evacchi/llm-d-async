package server

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/llm-d-incubation/llm-d-async/internal/logging"
	"github.com/llm-d-incubation/llm-d-async/pkg/async/inference/flowcontrol"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type ServerConfig struct {
	HealthPort          int
	MetricsPort         int
	MetricsEndpointAuth bool
}

type TLSConfig struct {
	CACert             string
	Cert               string
	Key                string
	InsecureSkipVerify bool
}

type WorkerConfig struct {
	Concurrency    int
	RequestTimeout time.Duration
	DrainTimeout   time.Duration
	PoolConfigFile string
}

type TransportOptions struct {
	Type                string
	Config              string
	ConfigFile          string
	MergePolicy         string
	BacklogPollInterval time.Duration
}

type ObservabilityConfig struct {
	Verbosity int
}

type PrometheusConfig struct {
	URL      string
	CacheTTL time.Duration
}

type Config struct {
	Server              ServerConfig
	TLS                 TLSConfig
	Worker              WorkerConfig
	Transport           TransportOptions
	Observability       ObservabilityConfig
	Prometheus          PrometheusConfig
	TransformConfigFile string
}

type Options struct {
	Config

	loggingOptions zap.Options
}

func NewOptions() *Options {
	return &Options{
		Config: Config{
			Server: ServerConfig{
				HealthPort:          8081,
				MetricsPort:         9090,
				MetricsEndpointAuth: true,
			},
			Worker: WorkerConfig{
				Concurrency:    8,
				RequestTimeout: 5 * time.Minute,
				DrainTimeout:   2 * time.Minute,
			},
			Transport: TransportOptions{
				Type:                "redis-pubsub",
				MergePolicy:         "random-robin",
				BacklogPollInterval: 15 * time.Second,
			},
			Observability: ObservabilityConfig{
				Verbosity: logging.DEFAULT,
			},
			Prometheus: PrometheusConfig{
				CacheTTL: flowcontrol.DefaultCacheTTL,
			},
		},
		loggingOptions: zap.Options{Development: true},
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.IntVarP(&o.Observability.Verbosity, "v", "v", o.Observability.Verbosity, "number for the log level verbosity")

	fs.IntVar(&o.Server.HealthPort, "health-port", o.Server.HealthPort, "The health probe port")
	fs.IntVar(&o.Server.MetricsPort, "metrics-port", o.Server.MetricsPort, "The metrics port")
	fs.BoolVar(&o.Server.MetricsEndpointAuth, "metrics-endpoint-auth", o.Server.MetricsEndpointAuth, "Enables authentication and authorization of the metrics endpoint")

	fs.IntVar(&o.Worker.Concurrency, "concurrency", o.Worker.Concurrency, "number of concurrent workers")
	fs.DurationVar(&o.Worker.RequestTimeout, "request-timeout", o.Worker.RequestTimeout, "timeout for individual inference requests")
	fs.DurationVar(&o.Worker.DrainTimeout, "drain-timeout", o.Worker.DrainTimeout, "maximum time to wait for in-flight requests to complete after SIGTERM")
	fs.StringVar(&o.Worker.PoolConfigFile, "pool-config-file", o.Worker.PoolConfigFile, "Path to the pools configuration JSON file")

	fs.StringVar(&o.Transport.Type, "transport", o.Transport.Type, "The transport implementation to use. Supported: redis-pubsub, redis-sortedset, gcp-pubsub")
	fs.StringVar(&o.Transport.Config, "transport-config", o.Transport.Config, "Inline JSON transport configuration")
	fs.StringVar(&o.Transport.ConfigFile, "transport-config-file", o.Transport.ConfigFile, "Path to transport configuration JSON file")
	fs.StringVar(&o.Transport.MergePolicy, "request-merge-policy", o.Transport.MergePolicy, "The request merge policy to use. Supported policies: random-robin")
	fs.DurationVar(&o.Transport.BacklogPollInterval, "metrics-backlog-poll-interval", o.Transport.BacklogPollInterval, "interval to poll the broker for queue backlog metrics (0 disables); only applies to flows that support it (redis-sortedset, gcp-pubsub)")

	fs.StringVar(&o.TLS.CACert, "tls-ca-cert", o.TLS.CACert, "Path to CA certificate file (PEM) for verifying the inference gateway")
	fs.StringVar(&o.TLS.Cert, "tls-cert", o.TLS.Cert, "Path to client certificate file (PEM) for mTLS")
	fs.StringVar(&o.TLS.Key, "tls-key", o.TLS.Key, "Path to client key file (PEM) for mTLS")
	fs.BoolVar(&o.TLS.InsecureSkipVerify, "tls-insecure-skip-verify", o.TLS.InsecureSkipVerify, "Skip TLS certificate verification (dev/test only)")

	fs.StringVar(&o.TransformConfigFile, "transform-config-file", o.TransformConfigFile, "Path to the body-transform plugins configuration JSON file (object with a requestTransforms array; empty disables transforms)")

	fs.StringVar(&o.Prometheus.URL, "prometheus-url", o.Prometheus.URL, "Prometheus server URL for metric-based gates (e.g., http://localhost:9090)")
	fs.DurationVar(&o.Prometheus.CacheTTL, "prometheus-cache-ttl", o.Prometheus.CacheTTL, "TTL for cached Prometheus metrics (e.g., 5s, 0s to disable)")

	// Zap logging flags (bridged from standard flag)
	goFlagSet := flag.NewFlagSet("", flag.ContinueOnError)
	o.loggingOptions.BindFlags(goFlagSet)
	fs.AddGoFlagSet(goFlagSet)
}

// LoggingOptions returns the zap options for initializing the logger.
func (o *Options) LoggingOptions() *zap.Options {
	return &o.loggingOptions
}

func (o *Options) Complete() error {
	hasTransportConfig := o.Transport.Config != "" || o.Transport.ConfigFile != ""
	hasPoolConfig := o.Worker.PoolConfigFile != ""

	if hasPoolConfig && !hasTransportConfig {
		return fmt.Errorf("pool-config-file can only be specified when transport config is also specified")
	}

	return nil
}

var (
	validTransports    = []string{"redis-pubsub", "redis-sortedset", "gcp-pubsub"}
	validMergePolicies = []string{"random-robin"}
)

func (o *Options) Validate() error {
	if !contains(validTransports, o.Transport.Type) {
		return fmt.Errorf("--transport must be one of: %s", strings.Join(validTransports, ", "))
	}
	if !contains(validMergePolicies, o.Transport.MergePolicy) {
		return fmt.Errorf("--request-merge-policy must be one of: %s", strings.Join(validMergePolicies, ", "))
	}
	if o.Transport.Config == "" && o.Transport.ConfigFile == "" {
		return fmt.Errorf("--transport-config or --transport-config-file is required")
	}
	if o.Transport.Config != "" && o.Transport.ConfigFile != "" {
		return fmt.Errorf("--transport-config and --transport-config-file are mutually exclusive")
	}
	if (o.TLS.Cert != "") != (o.TLS.Key != "") {
		return fmt.Errorf("both --tls-cert and --tls-key must be provided together")
	}
	return nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
