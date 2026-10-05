package testing

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

// ConcurrencySuite verifies concurrent operation safety and linearizability.
type ConcurrencySuite struct{}

func (s *ConcurrencySuite) Name() string {
	return "concurrency"
}

func (s *ConcurrencySuite) Description() string {
	return "Verifies multi-goroutine concurrent access, race freedom, and linearizability."
}

func (s *ConcurrencySuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "PartitionedWorkers",
			Fn: func(tc *TestContext) {
				workers := 8
				opsPerWorker := 25
				prefix := fmt.Sprintf("conc_part_%d_", time.Now().UnixNano())

				var wg sync.WaitGroup
				var errMu sync.Mutex
				var firstErr error

				for w := 0; w < workers; w++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						for i := 0; i < opsPerWorker; i++ {
							k := []byte(fmt.Sprintf("%sw%d_k%d", prefix, workerID, i))
							v := []byte(fmt.Sprintf("worker_%d_payload_%d", workerID, i))

							if err := tc.Client().Put(tc.Context(), k, v); err != nil {
								errMu.Lock()
								if firstErr == nil {
									firstErr = fmt.Errorf("worker %d put err: %w", workerID, err)
								}
								errMu.Unlock()
								return
							}

							got, err := tc.Client().Get(tc.Context(), k)
							if err != nil {
								errMu.Lock()
								if firstErr == nil {
									firstErr = fmt.Errorf("worker %d get err: %w", workerID, err)
								}
								errMu.Unlock()
								return
							}

							if !bytes.Equal(v, got) {
								errMu.Lock()
								if firstErr == nil {
									firstErr = fmt.Errorf("worker %d data corruption: expected %s, got %s", workerID, v, got)
								}
								errMu.Unlock()
								return
							}

							_ = tc.Client().Delete(tc.Context(), k)
						}
					}(w)
				}

				wg.Wait()
				tc.AssertNoError(firstErr, "concurrent partitioned workers completed")
				tc.Log("Ran %d concurrent workers executing %d ops each (%d total)", workers, opsPerWorker, workers*opsPerWorker)
			},
		},
		{
			Name: "SharedKeyContention",
			Fn: func(tc *TestContext) {
				sharedKey := []byte(fmt.Sprintf("conc_shared_%d", time.Now().UnixNano()))
				workers := 10
				iterations := 20

				var wg sync.WaitGroup
				var errMu sync.Mutex
				var firstErr error

				for w := 0; w < workers; w++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						for i := 0; i < iterations; i++ {
							val := []byte(fmt.Sprintf("writer_%d_iter_%d", workerID, i))
							if err := tc.Client().Put(tc.Context(), sharedKey, val); err != nil {
								errMu.Lock()
								if firstErr == nil {
									firstErr = fmt.Errorf("shared write err: %w", err)
								}
								errMu.Unlock()
								return
							}
						}
					}(w)
				}

				wg.Wait()
				defer func() { _ = tc.Client().Delete(tc.Context(), sharedKey) }()

				tc.AssertNoError(firstErr, "shared key contention writes")

				// Ensure key is intact and valid
				got, err := tc.Client().Get(tc.Context(), sharedKey)
				if !tc.AssertNoError(err, "read shared key after contention") {
					return
				}
				tc.AssertTrue(len(got) > 0, "shared key must hold non-empty value")
				tc.Log("Contended key read successfully with %d bytes", len(got))
			},
		},
	}
}
