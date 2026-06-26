package server

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/llm-d-incubation/llm-d-async/pipeline"
)

func loadPoolConfigBytes(w WorkerConfig) ([]byte, error) {
	if w.PoolConfigFile == "" {
		return json.Marshal([]pipeline.WorkerPoolConfig{{ID: "default", Workers: w.Concurrency}})
	}
	data, err := os.ReadFile(w.PoolConfigFile) // #nosec G304 -- path from trusted CLI flag
	if err != nil {
		return nil, fmt.Errorf("failed to read pool config file %q: %w", w.PoolConfigFile, err)
	}
	return data, nil
}

func loadTransportConfigBytes(transport TransportOptions) ([]byte, error) {
	if transport.Config != "" {
		return []byte(transport.Config), nil
	}
	data, err := os.ReadFile(transport.ConfigFile) // #nosec G304 -- path from trusted CLI flag
	if err != nil {
		return nil, fmt.Errorf("failed to read transport config file %q: %w", transport.ConfigFile, err)
	}
	return data, nil
}
