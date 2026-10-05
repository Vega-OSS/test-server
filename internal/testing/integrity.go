package testing

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"
)

// IntegritySuite verifies correctness against an in-memory oracle model.
type IntegritySuite struct{}

func (s *IntegritySuite) Name() string {
	return "integrity"
}

func (s *IntegritySuite) Description() string {
	return "Validates data correctness using a randomized model-based testing against an in-memory oracle."
}

func (s *IntegritySuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "ModelBasedOracleRandomized",
			Fn: func(tc *TestContext) {
				rng := rand.New(rand.NewSource(time.Now().UnixNano()))
				keyspaceSize := 30
				iterations := 150
				prefix := fmt.Sprintf("model_%d_", time.Now().UnixNano())

				oracle := make(map[string][]byte)
				keys := make([]string, keyspaceSize)
				for i := 0; i < keyspaceSize; i++ {
					keys[i] = fmt.Sprintf("%sk%02d", prefix, i)
				}

				defer func() {
					for _, k := range keys {
						_ = tc.Client().Delete(tc.Context(), []byte(k))
					}
				}()

				for iter := 0; iter < iterations; iter++ {
					k := keys[rng.Intn(keyspaceSize)]
					op := rng.Intn(10)

					switch {
					case op < 5: // 50% Put
						valLen := rng.Intn(128) + 1
						val := make([]byte, valLen)
						for j := 0; j < valLen; j++ {
							val[j] = byte(rng.Intn(256))
						}

						if err := tc.Client().Put(tc.Context(), []byte(k), val); err != nil {
							tc.Fail("iter %d: put %s failed: %v", iter, k, err)
							return
						}
						oracle[k] = val

					case op < 8: // 30% Get
						expectedVal, exists := oracle[k]
						gotVal, err := tc.Client().Get(tc.Context(), []byte(k))

						if exists {
							if err != nil {
								tc.Fail("iter %d: expected key %s to exist, got error: %v", iter, k, err)
								return
							}
							if !bytes.Equal(expectedVal, gotVal) {
								tc.Fail("iter %d: value mismatch for %s: expected len %d, got len %d", iter, k, len(expectedVal), len(gotVal))
								return
							}
						} else {
							if err == nil {
								tc.Fail("iter %d: expected key %s to return not found, but got value", iter, k)
								return
							}
							if !tc.AssertNotFound(err, fmt.Sprintf("iter %d: key %s should not exist", iter, k)) {
								return
							}
						}

					default: // 20% Delete
						if err := tc.Client().Delete(tc.Context(), []byte(k)); err != nil {
							tc.Fail("iter %d: delete %s failed: %v", iter, k, err)
							return
						}
						delete(oracle, k)
					}
				}

				// Final sweep: verify all remaining keys in oracle
				for k, expectedVal := range oracle {
					gotVal, err := tc.Client().Get(tc.Context(), []byte(k))
					if err != nil {
						tc.Fail("final sweep: key %s missing: %v", k, err)
						return
					}
					if !bytes.Equal(expectedVal, gotVal) {
						tc.Fail("final sweep: key %s value corrupt", k)
						return
					}
				}

				tc.Log("Completed %d randomized model transitions. All states matched oracle.", iterations)
			},
		},
	}
}
