package testing

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"time"
)

// CRUDSuite verifies CRUD edge cases and cross-protocol compatibility.
type CRUDSuite struct{}

func (s *CRUDSuite) Name() string {
	return "crud"
}

func (s *CRUDSuite) Description() string {
	return "Comprehensive validation of CRUD operations, empty values, large payloads, binary keys, and cross-protocol consistency."
}

func (s *CRUDSuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "EmptyValue",
			Fn: func(tc *TestContext) {
				key := []byte(fmt.Sprintf("crud_empty_%d", time.Now().UnixNano()))
				val := []byte{} // 0 bytes

				if !tc.AssertNoError(tc.Client().Put(tc.Context(), key, val), "put empty value") {
					return
				}
				defer func() { _ = tc.Client().Delete(tc.Context(), key) }()

				got, err := tc.Client().Get(tc.Context(), key)
				if !tc.AssertNoError(err, "get empty value") {
					return
				}
				tc.AssertEqual(val, got, "empty value must match exactly")
			},
		},
		{
			Name: "LargePayload",
			Fn: func(tc *TestContext) {
				key := []byte(fmt.Sprintf("crud_large_%d", time.Now().UnixNano()))
				// 256 KiB payload
				size := 256 * 1024
				val := make([]byte, size)
				_, _ = rand.Read(val)

				if !tc.AssertNoError(tc.Client().Put(tc.Context(), key, val), "put large payload") {
					return
				}
				defer func() { _ = tc.Client().Delete(tc.Context(), key) }()

				got, err := tc.Client().Get(tc.Context(), key)
				if !tc.AssertNoError(err, "get large payload") {
					return
				}
				tc.AssertIntEqual(len(val), len(got), "large payload length mismatch")
				tc.AssertTrue(bytes.Equal(val, got), "large payload contents mismatch")
				tc.Log("Stored and retrieved %d bytes successfully", size)
			},
		},
		{
			Name: "BinaryAndSpecialKeys",
			Fn: func(tc *TestContext) {
				// Keys containing slashes, null bytes, and non-printable characters
				specialKeys := [][]byte{
					[]byte("user/folder/file.txt"),
					[]byte("unicode/\u4e16\u754c/\U0001F600"),
					{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00},
					[]byte("key with spaces & symbols ?#="),
				}

				for i, k := range specialKeys {
					val := []byte(fmt.Sprintf("special_val_%d", i))
					if !tc.AssertNoError(tc.Client().Put(tc.Context(), k, val), fmt.Sprintf("put special key %d", i)) {
						return
					}
					got, err := tc.Client().Get(tc.Context(), k)
					if !tc.AssertNoError(err, fmt.Sprintf("get special key %d", i)) {
						_ = tc.Client().Delete(tc.Context(), k)
						return
					}
					tc.AssertEqual(val, got, fmt.Sprintf("value for special key %d mismatch", i))
					_ = tc.Client().Delete(tc.Context(), k)
				}
				tc.Log("Verified %d special/binary key formats", len(specialKeys))
			},
		},
		{
			Name: "OverwriteKey",
			Fn: func(tc *TestContext) {
				key := []byte(fmt.Sprintf("crud_overwrite_%d", time.Now().UnixNano()))
				val1 := []byte("first_version")
				val2 := []byte("second_updated_version")

				if !tc.AssertNoError(tc.Client().Put(tc.Context(), key, val1), "initial put") {
					return
				}
				defer func() { _ = tc.Client().Delete(tc.Context(), key) }()

				if !tc.AssertNoError(tc.Client().Put(tc.Context(), key, val2), "overwrite put") {
					return
				}

				got, err := tc.Client().Get(tc.Context(), key)
				if !tc.AssertNoError(err, "get after overwrite") {
					return
				}
				tc.AssertEqual(val2, got, "retrieved value must be the updated version")
			},
		},
		{
			Name: "NonExistentKeyReturnsNotFound",
			Fn: func(tc *TestContext) {
				key := []byte(fmt.Sprintf("crud_nonexistent_%d", time.Now().UnixNano()))
				_, err := tc.Client().Get(tc.Context(), key)
				tc.AssertNotFound(err, "getting non-existent key must return ErrNotFound")
			},
		},
		{
			Name: "CrossProtocolConsistency",
			Fn: func(tc *TestContext) {
				u := tc.Unified()
				if u == nil {
					tc.Log("Skipping cross-protocol test: unified client unavailable")
					return
				}

				// 1. Put via REST, Get via gRPC
				key1 := []byte(fmt.Sprintf("cross_rest_grpc_%d", time.Now().UnixNano()))
				val1 := []byte("written-via-rest-read-via-grpc")
				if !tc.AssertNoError(u.REST().Put(tc.Context(), key1, val1), "rest put") {
					return
				}
				defer func() { _ = u.GRPC().Delete(tc.Context(), key1) }()

				got1, err := u.GRPC().Get(tc.Context(), key1)
				if !tc.AssertNoError(err, "grpc get after rest put") {
					return
				}
				tc.AssertEqual(val1, got1, "rest -> grpc value mismatch")

				// 2. Put via gRPC, Get via REST
				key2 := []byte(fmt.Sprintf("cross_grpc_rest_%d", time.Now().UnixNano()))
				val2 := []byte("written-via-grpc-read-via-rest")
				if !tc.AssertNoError(u.GRPC().Put(tc.Context(), key2, val2), "grpc put") {
					return
				}
				defer func() { _ = u.REST().Delete(tc.Context(), key2) }()

				got2, err := u.REST().Get(tc.Context(), key2)
				if !tc.AssertNoError(err, "rest get after grpc put") {
					return
				}
				tc.AssertEqual(val2, got2, "grpc -> rest value mismatch")

				// 3. Delete via REST, Get via gRPC returns 404
				if !tc.AssertNoError(u.REST().Delete(tc.Context(), key2), "rest delete") {
					return
				}
				_, err = u.GRPC().Get(tc.Context(), key2)
				tc.AssertNotFound(err, "grpc get after rest delete must return not found")
				tc.Log("Cross-protocol REST <-> gRPC consistency validated")
			},
		},
	}
}
