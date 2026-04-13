package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Saturation Metric Dispatch Gate Integration", func() {
	var ctx context.Context

	ginkgo.BeforeEach(func() {
		ctx = context.Background()
		rdb.Del(ctx, integrationRequestQueue) //nolint:errcheck
		rdb.Del(ctx, integrationResultQueue)  //nolint:errcheck
		// Start with zero KV cache so saturation is low and the gate is open.
		setSimKvCache(simAdminURL, 0.0)
	})

	ginkgo.It("processes a message when KV cache is low (gate open)", func() {
		setSimKvCache(simAdminURL, 0.0)

		// Wait for EPP to reflect low saturation in Prometheus.
		waitForSaturation(promURL, func(v float64) bool { return v < 0.5 })

		msg := makeRequestMessage("integration-low-sat", 5*time.Minute)
		enqueueMessage(ctx, rdb, integrationRequestQueue, msg)

		gomega.Eventually(func() int64 {
			return getResultCount(ctx, rdb, integrationResultQueue)
		}, 60*time.Second, 1*time.Second).Should(gomega.BeNumerically(">=", 1))

		result := popResult(ctx, rdb, integrationResultQueue)
		gomega.Expect(result).NotTo(gomega.BeNil())
		gomega.Expect(result.Id).To(gomega.Equal("integration-low-sat"))
	})

	ginkgo.It("blocks messages when KV cache is high (gate closed)", func() {
		// Drive KV cache to 100% → EPP saturation >> threshold (0.7) → gate closed.
		setSimKvCache(simAdminURL, 1.0)

		// Wait for saturation to propagate through EPP → Prometheus.
		waitForSaturation(promURL, func(v float64) bool { return v >= 0.7 })

		msg := makeRequestMessage("integration-high-sat", 5*time.Minute)
		enqueueMessage(ctx, rdb, integrationRequestQueue, msg)

		gomega.Consistently(func() int64 {
			return getResultCount(ctx, rdb, integrationResultQueue)
		}, 10*time.Second, 1*time.Second).Should(gomega.Equal(int64(0)))

		// Drop KV cache → saturation falls → gate reopens → message processed.
		setSimKvCache(simAdminURL, 0.0)

		gomega.Eventually(func() int64 {
			return getResultCount(ctx, rdb, integrationResultQueue)
		}, 60*time.Second, 1*time.Second).Should(gomega.BeNumerically(">=", 1))

		result := popResult(ctx, rdb, integrationResultQueue)
		gomega.Expect(result).NotTo(gomega.BeNil())
		gomega.Expect(result.Id).To(gomega.Equal("integration-high-sat"))
	})

	ginkgo.It("resumes processing when KV cache drops", func() {
		setSimKvCache(simAdminURL, 1.0)
		waitForSaturation(promURL, func(v float64) bool { return v >= 0.7 })

		for i := 1; i <= 3; i++ {
			msg := makeRequestMessage(fmt.Sprintf("integration-resume-%d", i), 5*time.Minute)
			enqueueMessage(ctx, rdb, integrationRequestQueue, msg)
		}

		gomega.Consistently(func() int64 {
			return getResultCount(ctx, rdb, integrationResultQueue)
		}, 5*time.Second, 1*time.Second).Should(gomega.Equal(int64(0)))

		setSimKvCache(simAdminURL, 0.0)

		gomega.Eventually(func() int64 {
			return getResultCount(ctx, rdb, integrationResultQueue)
		}, 60*time.Second, 1*time.Second).Should(gomega.BeNumerically(">=", 3))
	})
})
