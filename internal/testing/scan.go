package testing

import (
	"fmt"
	"time"
)

// ScanSuite verifies range scan semantics, boundary conditions, and limit enforcement.
type ScanSuite struct{}

func (s *ScanSuite) Name() string {
	return "scan"
}

func (s *ScanSuite) Description() string {
	return "Verifies lexicographical ordered range scans, start/end boundaries, and limit truncation."
}

func (s *ScanSuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "BoundedRangeAndOrder",
			Fn: func(tc *TestContext) {
				prefix := fmt.Sprintf("scan_bound_%d_", time.Now().UnixNano())
				numKeys := 10

				// Insert ordered keys
				for i := 0; i < numKeys; i++ {
					k := []byte(fmt.Sprintf("%s%03d", prefix, i))
					v := []byte(fmt.Sprintf("val_%03d", i))
					if !tc.AssertNoError(tc.Client().Put(tc.Context(), k, v), "populate scan key") {
						return
					}
				}
				defer func() {
					for i := 0; i < numKeys; i++ {
						_ = tc.Client().Delete(tc.Context(), []byte(fmt.Sprintf("%s%03d", prefix, i)))
					}
				}()

				// Scan middle range [3, 7)
				start := []byte(fmt.Sprintf("%s%03d", prefix, 3))
				end := []byte(fmt.Sprintf("%s%03d", prefix, 7))

				results, err := tc.Client().Scan(tc.Context(), start, end, 100)
				if !tc.AssertNoError(err, "scan range [3, 7)") {
					return
				}

				if !tc.AssertIntEqual(4, len(results), "expected 4 items in range [3, 7)") {
					return
				}

				// Check ordering
				for i, item := range results {
					expectedKey := fmt.Sprintf("%s%03d", prefix, i+3)
					expectedVal := fmt.Sprintf("val_%03d", i+3)
					tc.AssertEqual([]byte(expectedKey), item.Key, fmt.Sprintf("item %d key mismatch", i))
					tc.AssertEqual([]byte(expectedVal), item.Value, fmt.Sprintf("item %d value mismatch", i))
				}
				tc.Log("Bounded range [3, 7) returned exact 4 ordered items")
			},
		},
		{
			Name: "LimitTruncation",
			Fn: func(tc *TestContext) {
				prefix := fmt.Sprintf("scan_limit_%d_", time.Now().UnixNano())
				numKeys := 15

				for i := 0; i < numKeys; i++ {
					k := []byte(fmt.Sprintf("%s%02d", prefix, i))
					v := []byte(fmt.Sprintf("val_%02d", i))
					if !tc.AssertNoError(tc.Client().Put(tc.Context(), k, v), "populate scan limit key") {
						return
					}
				}
				defer func() {
					for i := 0; i < numKeys; i++ {
						_ = tc.Client().Delete(tc.Context(), []byte(fmt.Sprintf("%s%02d", prefix, i)))
					}
				}()

				// Request limit = 5
				start := []byte(prefix)
				end := []byte(prefix + "\xff")
				limit := uint32(5)

				results, err := tc.Client().Scan(tc.Context(), start, end, limit)
				if !tc.AssertNoError(err, "scan with limit 5") {
					return
				}

				tc.AssertIntEqual(5, len(results), "scan must respect limit = 5")
				tc.Log("Scan limit enforcement confirmed: returned 5 out of 15 keys")
			},
		},
		{
			Name: "EmptyRange",
			Fn: func(tc *TestContext) {
				// Query non-existent range
				start := []byte("non_existent_range_start_zzzz")
				end := []byte("non_existent_range_end_zzzz")

				results, err := tc.Client().Scan(tc.Context(), start, end, 10)
				if !tc.AssertNoError(err, "empty scan query") {
					return
				}
				tc.AssertIntEqual(0, len(results), "empty range scan must return 0 results")
				tc.Log("Empty range scan correctly yielded 0 items")
			},
		},
	}
}
