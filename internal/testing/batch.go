package testing

import (
	"fmt"
	"strings"
	"time"

	"github.com/Vega-OSS/test-server/internal/client"
)

// BatchSuite validates atomic batch operations.
type BatchSuite struct{}

func (s *BatchSuite) Name() string {
	return "batch"
}

func (s *BatchSuite) Description() string {
	return "Verifies multi-operation atomic batches containing puts and deletes."
}

func (s *BatchSuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "AtomicBatchPut",
			Fn: func(tc *TestContext) {
				prefix := fmt.Sprintf("batch_put_%d_", time.Now().UnixNano())
				count := 5
				ops := make([]client.BatchOp, count)

				for i := 0; i < count; i++ {
					ops[i] = client.BatchOp{
						Kind:  client.BatchPut,
						Key:   []byte(fmt.Sprintf("%s%d", prefix, i)),
						Value: []byte(fmt.Sprintf("batch_val_%d", i)),
					}
				}

				if err := tc.Client().BatchWrite(tc.Context(), ops); err != nil {
					if strings.Contains(err.Error(), "Unimplemented") {
						tc.Log("BatchWrite RPC not implemented on %s transport (skipping)", tc.Client().Transport())
						return
					}
					tc.Fail("execute batch put failed: %v", err)
					return
				}
				defer func() {
					for i := 0; i < count; i++ {
						_ = tc.Client().Delete(tc.Context(), []byte(fmt.Sprintf("%s%d", prefix, i)))
					}
				}()

				// Assert all keys exist
				for i := 0; i < count; i++ {
					got, err := tc.Client().Get(tc.Context(), ops[i].Key)
					if !tc.AssertNoError(err, fmt.Sprintf("get batch key %d", i)) {
						return
					}
					tc.AssertEqual(ops[i].Value, got, fmt.Sprintf("batch key %d value mismatch", i))
				}
				tc.Log("Batch of %d puts applied and verified", count)
			},
		},
		{
			Name: "MixedBatchPutAndDelete",
			Fn: func(tc *TestContext) {
				prefix := fmt.Sprintf("batch_mix_%d_", time.Now().UnixNano())

				// Pre-populate key to be deleted
				delKey := []byte(prefix + "to_delete")
				if !tc.AssertNoError(tc.Client().Put(tc.Context(), delKey, []byte("original")), "pre-populate delKey") {
					return
				}

				newKey := []byte(prefix + "to_insert")
				newVal := []byte("brand_new_val")

				ops := []client.BatchOp{
					{
						Kind:  client.BatchPut,
						Key:   newKey,
						Value: newVal,
					},
					{
						Kind: client.BatchDelete,
						Key:  delKey,
					},
				}

				if err := tc.Client().BatchWrite(tc.Context(), ops); err != nil {
					if strings.Contains(err.Error(), "Unimplemented") {
						tc.Log("BatchWrite RPC not implemented on %s transport (skipping)", tc.Client().Transport())
						return
					}
					tc.Fail("execute mixed batch failed: %v", err)
					return
				}
				defer func() {
					_ = tc.Client().Delete(tc.Context(), newKey)
				}()

				// Assert newKey exists
				got, err := tc.Client().Get(tc.Context(), newKey)
				if !tc.AssertNoError(err, "get newKey") {
					return
				}
				tc.AssertEqual(newVal, got, "newKey value mismatch")

				// Assert delKey is gone
				_, err = tc.Client().Get(tc.Context(), delKey)
				tc.AssertNotFound(err, "delKey must be deleted in mixed batch")
				tc.Log("Mixed batch with Put and Delete committed atomically")
			},
		},
	}
}
