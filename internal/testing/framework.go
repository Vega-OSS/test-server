package testing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Vega-OSS/test-server/internal/client"
)

// TestStatus represents the outcome of a test case or suite.
type TestStatus string

const (
	StatusPassed  TestStatus = "PASSED"
	StatusFailed  TestStatus = "FAILED"
	StatusSkipped TestStatus = "SKIPPED"
	StatusRunning TestStatus = "RUNNING"
)

// TestCaseResult records the execution result of an individual test case.
type TestCaseResult struct {
	Name     string        `json:"name"`
	Status   TestStatus    `json:"status"`
	Duration time.Duration `json:"duration_ms"`
	Error    string        `json:"error,omitempty"`
	Logs     []string      `json:"logs,omitempty"`
}

// SuiteResult records the execution results of an entire test suite.
type SuiteResult struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      TestStatus       `json:"status"`
	Duration    time.Duration    `json:"duration_ms"`
	Total       int              `json:"total"`
	Passed      int              `json:"passed"`
	Failed      int              `json:"failed"`
	Cases       []TestCaseResult `json:"cases"`
}

// RunSummary represents a completed or in-progress test run.
type RunSummary struct {
	ID        string        `json:"id"`
	StartedAt time.Time     `json:"started_at"`
	Status    TestStatus    `json:"status"`
	Duration  time.Duration `json:"duration_ms"`
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Suites    []SuiteResult `json:"suites"`
	Error     string        `json:"error,omitempty"`
}

// TestContext provides test assertion helpers and context.
type TestContext struct {
	ctx     context.Context
	client  client.VegaClient
	unified *client.UnifiedClient
	logs    []string
	err     error
}

// NewTestContext creates a new test execution context.
func NewTestContext(ctx context.Context, c client.VegaClient, u *client.UnifiedClient) *TestContext {
	return &TestContext{
		ctx:     ctx,
		client:  c,
		unified: u,
	}
}

// Context returns the underlying context.
func (tc *TestContext) Context() context.Context {
	return tc.ctx
}

// Client returns the current protocol client under test.
func (tc *TestContext) Client() client.VegaClient {
	return tc.client
}

// Unified returns the unified client containing both gRPC and REST clients.
func (tc *TestContext) Unified() *client.UnifiedClient {
	return tc.unified
}

// Log records an informational log line for this test case.
func (tc *TestContext) Log(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	tc.logs = append(tc.logs, msg)
}

// Fail marks the test case as failed with an error message.
func (tc *TestContext) Fail(format string, args ...any) {
	if tc.err == nil {
		tc.err = fmt.Errorf(format, args...)
	}
}

// AssertNoError fails the test if err is not nil.
func (tc *TestContext) AssertNoError(err error, msg string) bool {
	if err != nil {
		tc.Fail("%s: unexpected error: %v", msg, err)
		return false
	}
	return true
}

// AssertEqual fails the test if expected != actual.
func (tc *TestContext) AssertEqual(expected, actual []byte, msg string) bool {
	if !bytes.Equal(expected, actual) {
		tc.Fail("%s: expected %q, got %q", msg, string(expected), string(actual))
		return false
	}
	return true
}

// AssertIntEqual fails the test if expected != actual.
func (tc *TestContext) AssertIntEqual(expected, actual int, msg string) bool {
	if expected != actual {
		tc.Fail("%s: expected %d, got %d", msg, expected, actual)
		return false
	}
	return true
}

// AssertTrue fails the test if cond is false.
func (tc *TestContext) AssertTrue(cond bool, msg string) bool {
	if !cond {
		tc.Fail("%s: expected condition to be true", msg)
		return false
	}
	return true
}

// AssertNotFound fails the test if err is not client.ErrNotFound.
func (tc *TestContext) AssertNotFound(err error, msg string) bool {
	if !errors.Is(err, client.ErrNotFound) {
		tc.Fail("%s: expected ErrNotFound, got %v", msg, err)
		return false
	}
	return true
}

// TestCase defines a single test routine.
type TestCase struct {
	Name string
	Fn   func(tc *TestContext)
}

// TestSuite represents a collection of test cases.
type TestSuite interface {
	Name() string
	Description() string
	Cases() []TestCase
}

// Registry stores and discovers available test suites.
type Registry struct {
	mu     sync.RWMutex
	suites map[string]TestSuite
}

// NewRegistry creates a new test suite registry.
func NewRegistry() *Registry {
	return &Registry{
		suites: make(map[string]TestSuite),
	}
}

// Register adds a test suite to the registry.
func (r *Registry) Register(suite TestSuite) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suites[suite.Name()] = suite
}

// Get returns a suite by name.
func (r *Registry) Get(name string) (TestSuite, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.suites[name]
	return s, ok
}

// List returns metadata of all registered suites.
func (r *Registry) List() []TestSuite {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]TestSuite, 0, len(r.suites))
	for _, s := range r.suites {
		list = append(list, s)
	}
	return list
}
