package testing

import (
	"context"
	"testing"

	"github.com/Vega-OSS/test-server/internal/client"
)

// mockClient implements client.VegaClient for testing the framework itself.
type mockClient struct {
	putErr    error
	getVal    []byte
	getErr    error
	deleteErr error
}

func (m *mockClient) Put(ctx context.Context, key, val []byte) error { return m.putErr }
func (m *mockClient) Get(ctx context.Context, key []byte) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getVal, nil
}
func (m *mockClient) Delete(ctx context.Context, key []byte) error { return m.deleteErr }
func (m *mockClient) Scan(ctx context.Context, start, end []byte, limit uint32) ([]client.KV, error) {
	return []client.KV{{Key: start, Value: m.getVal}}, nil
}
func (m *mockClient) BatchWrite(ctx context.Context, ops []client.BatchOp) error { return nil }
func (m *mockClient) Stats(ctx context.Context) (map[string]int64, error) {
	return map[string]int64{"test_metric": 1}, nil
}
func (m *mockClient) Health(ctx context.Context) error { return nil }
func (m *mockClient) Close() error                     { return nil }
func (m *mockClient) Transport() string                { return "mock" }

func TestFrameworkAssertions(t *testing.T) {
	mc := &mockClient{getVal: []byte("val")}
	tc := NewTestContext(context.Background(), mc, nil)

	// AssertNoError
	if !tc.AssertNoError(nil, "nil error") {
		t.Fatal("expected AssertNoError to succeed")
	}

	// AssertEqual
	if !tc.AssertEqual([]byte("abc"), []byte("abc"), "equal bytes") {
		t.Fatal("expected AssertEqual to succeed")
	}

	// AssertIntEqual
	if !tc.AssertIntEqual(42, 42, "equal ints") {
		t.Fatal("expected AssertIntEqual to succeed")
	}

	// AssertTrue
	if !tc.AssertTrue(true, "true condition") {
		t.Fatal("expected AssertTrue to succeed")
	}

	// AssertNotFound
	if !tc.AssertNotFound(client.ErrNotFound, "not found error") {
		t.Fatal("expected AssertNotFound to succeed")
	}

	// Fail condition check
	tc.Fail("deliberate failure: %s", "test")
	if tc.err == nil {
		t.Fatal("expected tc.err to be set")
	}
}

type dummySuite struct{}

func (d *dummySuite) Name() string        { return "dummy" }
func (d *dummySuite) Description() string { return "dummy suite" }
func (d *dummySuite) Cases() []TestCase {
	return []TestCase{
		{
			Name: "PassCase",
			Fn: func(tc *TestContext) {
				tc.Log("all good")
			},
		},
		{
			Name: "FailCase",
			Fn: func(tc *TestContext) {
				tc.Fail("failed intentionally")
			},
		},
	}
}

func TestRegistryAndRunner(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummySuite{})

	s, ok := reg.Get("dummy")
	if !ok || s.Name() != "dummy" {
		t.Fatal("failed to get registered suite")
	}

	if len(reg.List()) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(reg.List()))
	}
}
