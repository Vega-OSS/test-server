package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Vega-OSS/test-server/internal/client"
)

// RunConfig configures a test execution request.
type RunConfig struct {
	Suites      []string `json:"suites"`
	Protocol    string   `json:"protocol"` // "grpc", "rest", or "both" (default)
	Concurrency int      `json:"concurrency,omitempty"`
}

// Runner coordinates test execution and retains run history.
type Runner struct {
	registry *Registry
	client   *client.UnifiedClient
	mu       sync.RWMutex
	history  []*RunSummary
	active   map[string]context.CancelFunc
}

// NewRunner creates a new test runner.
func NewRunner(r *Registry, u *client.UnifiedClient) *Runner {
	return &Runner{
		registry: r,
		client:   u,
		history:  make([]*RunSummary, 0, 50),
		active:   make(map[string]context.CancelFunc),
	}
}

// Run executes the requested test suites synchronously or asynchronously.
func (runner *Runner) Run(parentCtx context.Context, cfg RunConfig) (*RunSummary, error) {
	runID := fmt.Sprintf("run_%d", time.Now().UnixNano())

	ctx, cancel := context.WithCancel(parentCtx)
	runner.mu.Lock()
	runner.active[runID] = cancel
	summary := &RunSummary{
		ID:        runID,
		StartedAt: time.Now(),
		Status:    StatusRunning,
		Suites:    make([]SuiteResult, 0),
	}
	runner.history = append([]*RunSummary{summary}, runner.history...)
	if len(runner.history) > 100 {
		runner.history = runner.history[:100]
	}
	runner.mu.Unlock()

	defer func() {
		runner.mu.Lock()
		delete(runner.active, runID)
		runner.mu.Unlock()
	}()

	suitesToRun := runner.resolveSuites(cfg.Suites)
	protocols := runner.resolveProtocols(cfg.Protocol)

	startTime := time.Now()
	allPassed := true

	for _, proto := range protocols {
		var activeClient client.VegaClient
		if proto == "grpc" {
			activeClient = runner.client.GRPC()
		} else {
			activeClient = runner.client.REST()
		}

		for _, suite := range suitesToRun {
			select {
			case <-ctx.Done():
				summary.Status = StatusFailed
				summary.Error = "cancelled"
				summary.Duration = time.Since(startTime)
				return summary, ctx.Err()
			default:
			}

			sResult := runner.executeSuite(ctx, suite, activeClient, runner.client, proto)
			summary.Suites = append(summary.Suites, sResult)
			summary.Total += sResult.Total
			summary.Passed += sResult.Passed
			summary.Failed += sResult.Failed
			if sResult.Status != StatusPassed {
				allPassed = false
			}
		}
	}

	summary.Duration = time.Since(startTime)
	if allPassed {
		summary.Status = StatusPassed
	} else {
		summary.Status = StatusFailed
	}

	return summary, nil
}

func (runner *Runner) executeSuite(ctx context.Context, suite TestSuite, c client.VegaClient, u *client.UnifiedClient, proto string) SuiteResult {
	suiteName := fmt.Sprintf("%s [%s]", suite.Name(), proto)
	sResult := SuiteResult{
		Name:        suiteName,
		Description: suite.Description(),
		Status:      StatusPassed,
		Cases:       make([]TestCaseResult, 0, len(suite.Cases())),
	}

	sStart := time.Now()

	for _, tc := range suite.Cases() {
		select {
		case <-ctx.Done():
			cResult := TestCaseResult{
				Name:   tc.Name,
				Status: StatusFailed,
				Error:  "cancelled by context",
			}
			sResult.Cases = append(sResult.Cases, cResult)
			sResult.Failed++
			sResult.Status = StatusFailed
			continue
		default:
		}

		testCtx := NewTestContext(ctx, c, u)
		tStart := time.Now()

		func() {
			defer func() {
				if r := recover(); r != nil {
					testCtx.Fail("panic during test execution: %v", r)
				}
			}()
			tc.Fn(testCtx)
		}()

		cDuration := time.Since(tStart)
		cResult := TestCaseResult{
			Name:     tc.Name,
			Duration: cDuration,
			Logs:     testCtx.logs,
		}

		if testCtx.err != nil {
			cResult.Status = StatusFailed
			cResult.Error = testCtx.err.Error()
			sResult.Failed++
			sResult.Status = StatusFailed
		} else {
			cResult.Status = StatusPassed
			sResult.Passed++
		}

		sResult.Cases = append(sResult.Cases, cResult)
		sResult.Total++
	}

	sResult.Duration = time.Since(sStart)
	return sResult
}

func (runner *Runner) resolveSuites(requested []string) []TestSuite {
	if len(requested) == 0 {
		return runner.registry.List()
	}

	for _, s := range requested {
		if s == "all" {
			return runner.registry.List()
		}
	}

	var resolved []TestSuite
	for _, name := range requested {
		if suite, ok := runner.registry.Get(name); ok {
			resolved = append(resolved, suite)
		}
	}
	return resolved
}

func (runner *Runner) resolveProtocols(requested string) []string {
	switch requested {
	case "grpc":
		return []string{"grpc"}
	case "rest", "http":
		return []string{"rest"}
	case "both", "":
		return []string{"grpc", "rest"}
	default:
		return []string{"grpc", "rest"}
	}
}

// History returns past test runs.
func (runner *Runner) History() []*RunSummary {
	runner.mu.RLock()
	defer runner.mu.RUnlock()
	out := make([]*RunSummary, len(runner.history))
	copy(out, runner.history)
	return out
}

// GetRun returns a specific run summary by ID.
func (runner *Runner) GetRun(id string) (*RunSummary, bool) {
	runner.mu.RLock()
	defer runner.mu.RUnlock()
	for _, r := range runner.history {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

// CancelRun aborts an active test run by ID.
func (runner *Runner) CancelRun(id string) bool {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if cancel, ok := runner.active[id]; ok {
		cancel()
		delete(runner.active, id)
		return true
	}
	return false
}
