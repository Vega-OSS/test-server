package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	vegatesting "github.com/Vega-OSS/test-server/internal/testing"
)

func TestServerEndpoints(t *testing.T) {
	reg := vegatesting.NewRegistry()
	vegatesting.RegisterDefaultSuites(reg)

	runner := vegatesting.NewRunner(reg, nil)
	srv := New(Options{
		Addr:     ":0",
		Runner:   runner,
		Registry: reg,
	})

	mux := srv.HTTPServer().Handler

	// 1. Healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", rec.Code)
	}

	// 2. Dashboard root /
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected / 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %s", rec.Header().Get("Content-Type"))
	}

	// 3. List suites
	req = httptest.NewRequest(http.MethodGet, "/api/v1/test/suites", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/test/suites 200, got %d", rec.Code)
	}
	var suites []suiteInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &suites); err != nil {
		t.Fatalf("failed to decode suites: %v", err)
	}
	if len(suites) < 7 {
		t.Fatalf("expected at least 7 suites registered, got %d", len(suites))
	}

	// 4. List history
	req = httptest.NewRequest(http.MethodGet, "/api/v1/test/history", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/test/history 200, got %d", rec.Code)
	}
}
