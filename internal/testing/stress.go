package testing

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// StressSuite runs a burst load test and computes latency percentiles.
type StressSuite struct{}

func (s *StressSuite) Name() string {
	return "stress"
}

func (s *StressSuite) Description() string {
	return "Executes high-throughput concurrent operations and reports QPS and latency percentiles (p50, p90, p95, p99)."
}

func (s *StressSuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "BurstThroughputAndLatency",
			Fn: func(tc *TestContext) {
				workers := 8
				totalOps := 200
				opsPerWorker := totalOps / workers
				prefix := fmt.Sprintf("stress_%d_", time.Now().UnixNano())

				latencies := make([]time.Duration, 0, totalOps*2)
				var latMu sync.Mutex
				var errCount atomic.Int64
				var successCount atomic.Int64

				start := time.Now()
				var wg sync.WaitGroup

				for w := 0; w < workers; w++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						workerLats := make([]time.Duration, 0, opsPerWorker*2)

						for i := 0; i < opsPerWorker; i++ {
							k := []byte(fmt.Sprintf("%sw%d_%d", prefix, workerID, i))
							v := []byte(fmt.Sprintf("stress_payload_%d", i))

							// Put
							t0 := time.Now()
							if err := tc.Client().Put(tc.Context(), k, v); err != nil {
								errCount.Add(1)
							} else {
								successCount.Add(1)
								workerLats = append(workerLats, time.Since(t0))
							}

							// Get
							t1 := time.Now()
							if _, err := tc.Client().Get(tc.Context(), k); err != nil {
								errCount.Add(1)
							} else {
								successCount.Add(1)
								workerLats = append(workerLats, time.Since(t1))
							}

							_ = tc.Client().Delete(tc.Context(), k)
						}

						latMu.Lock()
						latencies = append(latencies, workerLats...)
						latMu.Unlock()
					}(w)
				}

				wg.Wait()
				elapsed := time.Since(start)

				if errCount.Load() > 0 {
					tc.Fail("%d operations failed during stress test", errCount.Load())
					return
				}

				if len(latencies) == 0 {
					tc.Fail("no latency measurements recorded")
					return
				}

				sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
				n := len(latencies)
				p50 := latencies[n*50/100]
				p90 := latencies[n*90/100]
				p95 := latencies[n*95/100]
				p99 := latencies[n*99/100]
				qps := float64(successCount.Load()) / elapsed.Seconds()

				tc.Log("Stress run completed: %d ops in %v (%.2f ops/sec)", successCount.Load(), elapsed.Round(time.Millisecond), qps)
				tc.Log("Latency Distribution: p50=%v | p90=%v | p95=%v | p99=%v", p50, p90, p95, p99)
			},
		},
	}
}
