package testing

import (
	"fmt"
	"time"
)

// SmokeSuite verifies baseline liveness and single-key lifecycle.
type SmokeSuite struct{}

func (s *SmokeSuite) Name() string {
	return "smoke"
}

func (s *SmokeSuite) Description() string {
	return "Verifies baseline connectivity, liveness, and basic Put/Get/Delete lifecycle."
}

func (s *SmokeSuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "HealthAndLiveness",
			Fn: func(tc *TestContext) {
				err := tc.Client().Health(tc.Context())
				tc.AssertNoError(err, "health check failed")
				tc.Log("Health check succeeded for transport: %s", tc.Client().Transport())
			},
		},
		{
			Name: "BasicRoundTrip",
			Fn: func(tc *TestContext) {
				key := []byte(fmt.Sprintf("smoke_test_%d", time.Now().UnixNano()))
				val := []byte("hello-vegadb-world")

				// 1. Put
				err := tc.Client().Put(tc.Context(), key, val)
				if !tc.AssertNoError(err, "put failed") {
					return
				}
				tc.Log("Key stored successfully: %s", string(key))

				// 2. Get
				gotVal, err := tc.Client().Get(tc.Context(), key)
				if !tc.AssertNoError(err, "get failed") {
					return
				}
				tc.AssertEqual(val, gotVal, "retrieved value does not match")
				tc.Log("Key verified successfully")

				// 3. Delete
				err = tc.Client().Delete(tc.Context(), key)
				if !tc.AssertNoError(err, "delete failed") {
					return
				}
				tc.Log("Key deleted successfully")

				// 4. Assert Not Found
				_, err = tc.Client().Get(tc.Context(), key)
				tc.AssertNotFound(err, "deleted key must return not found")
			},
		},
		{
			Name: "StatsEndpoint",
			Fn: func(tc *TestContext) {
				stats, err := tc.Client().Stats(tc.Context())
				if !tc.AssertNoError(err, "stats call failed") {
					return
				}
				tc.AssertTrue(stats != nil, "stats map must not be nil")
				tc.Log("Server reported %d metric counters", len(stats))
			},
		},
	}
}
