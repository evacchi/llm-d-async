package redis

import (
	"encoding/json"
	"fmt"
	"os"
)

// PubSubConfig is the transport config for the Redis pub/sub flow.
// It is parsed from JSON provided via --transport-config or --transport-config-file.
type PubSubConfig struct {
	URL             string        `json:"url,omitempty"`
	RetryQueueName  string        `json:"retry_queue_name,omitempty"`
	ResultQueueName string        `json:"result_queue_name,omitempty"`
	EnableTracing   bool          `json:"enable_tracing,omitempty"`
	Queues          []QueueConfig `json:"queues"`
}

// LoadPubSubConfig parses, applies env overrides/defaults, and validates a PubSubConfig.
func LoadPubSubConfig(data []byte) (*PubSubConfig, error) {
	var cfg PubSubConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse redis-pubsub transport config: %w", err)
	}
	cfg.ApplyEnvOverrides()
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis-pubsub transport config: %w", err)
	}
	return &cfg, nil
}

func (c *PubSubConfig) ApplyEnvOverrides() {
	if envURL := os.Getenv("REDIS_URL"); envURL != "" {
		c.URL = envURL
	}
}

func (c *PubSubConfig) ApplyDefaults() {
	if c.RetryQueueName == "" {
		c.RetryQueueName = "retry-sortedset"
	}
	if c.ResultQueueName == "" {
		c.ResultQueueName = "result-queue"
	}
	for i := range c.Queues {
		if c.Queues[i].RequestPathURL == "" {
			c.Queues[i].RequestPathURL = "/v1/completions"
		}
		if c.Queues[i].WorkerPoolID == "" {
			c.Queues[i].WorkerPoolID = "default"
		}
	}
}

func (c *PubSubConfig) Validate() error {
	if len(c.Queues) == 0 {
		return fmt.Errorf("at least one queue must be configured")
	}
	for _, q := range c.Queues {
		if q.QueueName == "" {
			return fmt.Errorf("queue_name is required for each queue")
		}
		if q.IGWBaseURL == "" {
			return fmt.Errorf("queue %q: igw_base_url must be specified", q.QueueName)
		}
	}
	return nil
}

// SortedSetConfig is the transport config for the Redis sorted-set flow.
// It is parsed from JSON provided via --transport-config or --transport-config-file.
type SortedSetConfig struct {
	URL             string                 `json:"url,omitempty"`
	ResultQueueName string                 `json:"result_queue_name,omitempty"`
	PollIntervalMs  int                    `json:"poll_interval_ms,omitempty"`
	BatchSize       int                    `json:"batch_size,omitempty"`
	EnableTracing   bool                   `json:"enable_tracing,omitempty"`
	Queues          []SortedSetQueueConfig `json:"queues"`
}

// LoadSortedSetConfig parses, applies env overrides/defaults, and validates a SortedSetConfig.
func LoadSortedSetConfig(data []byte) (*SortedSetConfig, error) {
	var cfg SortedSetConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse redis-sortedset transport config: %w", err)
	}
	cfg.ApplyEnvOverrides()
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis-sortedset transport config: %w", err)
	}
	return &cfg, nil
}

func (c *SortedSetConfig) ApplyEnvOverrides() {
	if envURL := os.Getenv("REDIS_URL"); envURL != "" {
		c.URL = envURL
	}
}

func (c *SortedSetConfig) ApplyDefaults() {
	if c.ResultQueueName == "" {
		c.ResultQueueName = "result-list"
	}
	if c.PollIntervalMs == 0 {
		c.PollIntervalMs = 1000
	}
	if c.BatchSize == 0 {
		c.BatchSize = 10
	}
	for i := range c.Queues {
		if c.Queues[i].RequestPathURL == "" {
			c.Queues[i].RequestPathURL = "/v1/completions"
		}
		if c.Queues[i].WorkerPoolID == "" {
			c.Queues[i].WorkerPoolID = "default"
		}
		if c.Queues[i].ID == "" {
			c.Queues[i].ID = c.Queues[i].QueueName
		}
	}
}

func (c *SortedSetConfig) Validate() error {
	if len(c.Queues) == 0 {
		return fmt.Errorf("at least one queue must be configured")
	}
	seenID := make(map[string]bool, len(c.Queues))
	seenQueue := make(map[string]bool, len(c.Queues))
	for _, q := range c.Queues {
		if q.QueueName == "" {
			return fmt.Errorf("queue_name is required for each queue")
		}
		if q.IGWBaseURL == "" {
			return fmt.Errorf("queue %q: igw_base_url must be specified", q.QueueName)
		}
		if seenID[q.ID] {
			return fmt.Errorf("duplicate queue id %q", q.ID)
		}
		seenID[q.ID] = true
		if seenQueue[q.QueueName] {
			return fmt.Errorf("duplicate queue_name %q", q.QueueName)
		}
		seenQueue[q.QueueName] = true
	}
	return nil
}
