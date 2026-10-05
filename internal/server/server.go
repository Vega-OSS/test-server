package server

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Vega-OSS/test-server/internal/client"
	"github.com/Vega-OSS/test-server/internal/testing"
)

//go:embed dashboard.html
var dashboardHTML []byte

// Options configures the test server.
type Options struct {
	Addr          string
	UnifiedClient *client.UnifiedClient
	Runner        *testing.Runner
	Registry      *testing.Registry
	Logger        *slog.Logger
}

// Server encapsulates the HTTP test server and API endpoints.
type Server struct {
	httpServer *http.Server
	client     *client.UnifiedClient
	runner     *testing.Runner
	registry   *testing.Registry
	logger     *slog.Logger
}

// New creates and configures the test HTTP server.
func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		client:   opts.UnifiedClient,
		runner:   opts.Runner,
		registry: opts.Registry,
		logger:   logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /vega/health", s.handleVegaHealth)
	mux.HandleFunc("GET /api/v1/test/suites", s.handleListSuites)
	mux.HandleFunc("POST /api/v1/test/run", s.handleRunTests)
	mux.HandleFunc("GET /api/v1/test/history", s.handleListHistory)
	mux.HandleFunc("GET /api/v1/test/history/{id}", s.handleGetHistory)
	mux.HandleFunc("POST /api/v1/test/cancel/{id}", s.handleCancelTest)
	mux.HandleFunc("GET /", s.handleDashboard)

	s.httpServer = &http.Server{
		Addr:              opts.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

// HTTPServer returns the underlying *http.Server.
func (s *Server) HTTPServer() *http.Server {
	return s.httpServer
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleVegaHealth(w http.ResponseWriter, r *http.Request) {
	status := s.client.CheckHealth(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

type suiteInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CasesCount  int    `json:"cases_count"`
}

func (s *Server) handleListSuites(w http.ResponseWriter, _ *http.Request) {
	suites := s.registry.List()
	out := make([]suiteInfo, len(suites))
	for i, suite := range suites {
		out[i] = suiteInfo{
			Name:        suite.Name(),
			Description: suite.Description(),
			CasesCount:  len(suite.Cases()),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleRunTests(w http.ResponseWriter, r *http.Request) {
	var cfg testing.RunConfig
	if r.Header.Get("Content-Type") == "application/json" && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
			return
		}
	}

	s.logger.Info("triggering test run", "suites", cfg.Suites, "protocol", cfg.Protocol)
	summary, err := s.runner.Run(r.Context(), cfg)
	if err != nil && !errors.Is(err, r.Context().Err()) {
		s.logger.Error("test execution failed", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleListHistory(w http.ResponseWriter, _ *http.Request) {
	history := s.runner.History()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(history)
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	summary, ok := s.runner.GetRun(id)
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleCancelTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.runner.CancelRun(id) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cancelled\n"))
		return
	}
	http.Error(w, "run not active or not found", http.StatusNotFound)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(dashboardHTML)
}
