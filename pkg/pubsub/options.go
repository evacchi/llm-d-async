package pubsub

import (
	"encoding/json"
	"fmt"
)

// Config is the transport config for the GCP PubSub flow.
// It is parsed from JSON provided via --transport-config or --transport-config-file.
type Config struct {
	ProjectID     string        `json:"project_id"`
	ResultTopicID string        `json:"result_topic_id"`
	BatchSize     int           `json:"batch_size,omitempty"`
	Topics        []TopicConfig `json:"topics"`
}

// LoadConfig parses, applies defaults, and validates a GCP PubSub Config.
func LoadConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse gcp-pubsub transport config: %w", err)
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid gcp-pubsub transport config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.BatchSize == 0 {
		c.BatchSize = 10
	}
	for i := range c.Topics {
		if c.Topics[i].RequestPathURL == "" {
			c.Topics[i].RequestPathURL = "/v1/completions"
		}
		if c.Topics[i].WorkerPoolID == "" {
			c.Topics[i].WorkerPoolID = "default"
		}
	}
}

func (c *Config) Validate() error {
	if c.ProjectID == "" {
		return fmt.Errorf("project_id is required for gcp-pubsub transport")
	}
	if len(c.Topics) == 0 {
		return fmt.Errorf("at least one topic must be configured")
	}
	for _, t := range c.Topics {
		if t.SubscriberID == "" {
			return fmt.Errorf("subscriber_id is required for each topic")
		}
		if t.IGWBaseURL == "" {
			return fmt.Errorf("topic subscriber %q: igw_base_url must be specified", t.SubscriberID)
		}
	}
	return nil
}
