package pipeline

import (
	"encoding/json"
	"fmt"
)

// WorkerPoolConfig defines the configuration for a worker pool,
// specifying the concurrency limit (number of workers) and its ID.
type WorkerPoolConfig struct {
	ID      string `json:"id"`
	Workers int    `json:"workers"`
}

// LoadWorkerPools parses and validates worker pool configurations from JSON bytes.
func LoadWorkerPools(data []byte) ([]WorkerPoolConfig, error) {
	var pools []WorkerPoolConfig
	if err := json.Unmarshal(data, &pools); err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	seenIDs := make(map[string]bool)
	for i, pool := range pools {
		if pool.ID == "" {
			return nil, fmt.Errorf("pool config at index %d has an empty ID", i)
		}
		if pool.Workers <= 0 {
			return nil, fmt.Errorf("pool %q must have at least 1 worker", pool.ID)
		}
		if seenIDs[pool.ID] {
			return nil, fmt.Errorf("duplicate pool ID found: %q", pool.ID)
		}
		seenIDs[pool.ID] = true
	}

	return pools, nil
}
