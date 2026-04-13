package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"

	"github.com/llm-d-incubation/llm-d-async/pkg/async/api"
)

const (
	integrationRequestQueue = "integration-request-sortedset"
	integrationResultQueue  = "integration-result-list"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func enqueueMessage(ctx context.Context, rdb *redis.Client, queue string, msg api.RequestMessage) {
	data, err := json.Marshal(msg)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	err = rdb.ZAdd(ctx, queue, redis.Z{
		Score:  parseDeadline(msg.DeadlineUnixSec),
		Member: string(data),
	}).Err()
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
}

func parseDeadline(deadline string) float64 {
	var d float64
	_, err := fmt.Sscanf(deadline, "%f", &d)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	return d
}

func getResultCount(ctx context.Context, rdb *redis.Client, queue string) int64 {
	n, err := rdb.LLen(ctx, queue).Result()
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	return n
}

func popResult(ctx context.Context, rdb *redis.Client, queue string) *api.ResultMessage {
	val, err := rdb.RPop(ctx, queue).Result()
	if err == redis.Nil {
		return nil
	}
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	var msg api.ResultMessage
	gomega.ExpectWithOffset(1, json.Unmarshal([]byte(val), &msg)).To(gomega.Succeed())
	return &msg
}

func makeRequestMessage(id string, deadlineOffset time.Duration) api.RequestMessage {
	deadline := time.Now().Add(deadlineOffset)
	return api.RequestMessage{
		Id:              id,
		CreatedUnixSec:  strconv.FormatInt(time.Now().Unix(), 10),
		DeadlineUnixSec: fmt.Sprintf("%d", deadline.Unix()),
		Payload:         map[string]any{"model": "test-model", "prompt": "test prompt"},
	}
}

// setSimKvCache drives the vLLM simulator's KV cache usage to the given value [0.0, 1.0].
func setSimKvCache(simAdminURL string, value float64) {
	body, _ := json.Marshal(map[string]any{
		"kv-cache-usage":   value,
		"waiting-requests": 0,
	})
	req, err := http.NewRequest(http.MethodPost, simAdminURL+"/fake_metrics", bytes.NewReader(body))
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	defer resp.Body.Close() //nolint:errcheck
	gomega.ExpectWithOffset(1, resp.StatusCode).To(gomega.BeElementOf(http.StatusOK, http.StatusNoContent))
}

// sendProbeRequest sends a minimal inference request through Envoy → EPP to trigger
// the EPP's flow control admission controller, which records the saturation metric.
func sendProbeRequest(envoyURL string) {
	body := []byte(`{"model":"test-model","prompt":"probe"}`)
	req, err := http.NewRequest(http.MethodPost, envoyURL+"/v1/completions", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close() //nolint:errcheck
}

// queryPromSaturation queries Prometheus for the EPP pool saturation metric,
// which is recorded when requests flow through EPP's flow control admission layer.
// Returns -1 if the metric is not yet available.
func queryPromSaturation(promURL string) float64 {
	query := `inference_extension_flow_control_pool_saturation{inference_pool="e2e-pool"}`
	resp, err := httpClient.Get(promURL + "/api/v1/query?query=" + query)
	if err != nil {
		return -1
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Value [2]any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return -1
	}
	if result.Status != "success" || len(result.Data.Result) == 0 {
		return -1
	}
	v, err := strconv.ParseFloat(result.Data.Result[0].Value[1].(string), 64)
	if err != nil {
		return -1
	}
	return v
}

// waitForSaturation polls Prometheus until pred is satisfied or the timeout elapses.
// It sends probe requests through Envoy on each poll to trigger EPP's flow control
// admission controller, which records the saturation metric.
func waitForSaturation(promURL, envoyURL string, pred func(float64) bool) {
	gomega.EventuallyWithOffset(1, func() bool {
		sendProbeRequest(envoyURL)
		v := queryPromSaturation(promURL)
		return v >= 0 && pred(v)
	}, 60*time.Second, 2*time.Second).Should(gomega.BeTrue(), "waiting for saturation to satisfy condition")
}
