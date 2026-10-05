package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Vega-OSS/test-server/internal/client"
	"github.com/Vega-OSS/test-server/internal/server"
	"github.com/Vega-OSS/test-server/internal/testing"
)

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	var (
		port           = flag.String("port", getEnv("PORT", ":8081"), "Test server HTTP listen address")
		vegaGRPC       = flag.String("vega-grpc", getEnv("VEGA_GRPC_ADDR", "127.0.0.1:50051"), "VegaDB gRPC address")
		vegaHTTP       = flag.String("vega-http", getEnv("VEGA_HTTP_ADDR", "http://127.0.0.1:8080"), "VegaDB HTTP REST URL")
		grpcPoolSize   = flag.Int("grpc-pool", getEnvInt("GRPC_POOL_SIZE", 4), "gRPC client connection pool size")
		runOnStartup   = flag.String("run-on-startup", getEnv("RUN_ON_STARTUP", ""), "Run test suites immediately on startup (e.g. 'smoke' or 'all')")
		autoExit       = flag.Bool("auto-exit", getEnvBool("AUTO_EXIT", false), "Exit process after executing startup tests (code 0 on success, 1 on failure)")
		logLevelStr    = flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Logging verbosity (debug|info|warn|error)")
	)
	flag.Parse()

	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(*logLevelStr),
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	listenAddr := *port
	if !strings.Contains(listenAddr, ":") {
		listenAddr = ":" + listenAddr
	}

	logger.Info("starting VegaDB Test Server",
		"port", listenAddr,
		"vega_grpc", *vegaGRPC,
		"vega_http", *vegaHTTP,
	)

	// 1. Initialize Unified Client
	cfg := client.Config{
		GRPCAddr:     *vegaGRPC,
		RESTAddr:     *vegaHTTP,
		GRPCPoolSize: *grpcPoolSize,
		Timeout:      10 * time.Second,
	}

	unifiedClient, err := client.NewUnifiedClient(cfg)
	if err != nil {
		logger.Error("failed to create client to VegaDB", "error", err)
		os.Exit(1)
	}
	defer unifiedClient.Close()

	// 2. Initialize Registry and Test Suites
	registry := testing.NewRegistry()
	testing.RegisterDefaultSuites(registry)

	// 3. Initialize Runner
	runner := testing.NewRunner(registry, unifiedClient)

	// 4. Initialize HTTP Server
	srv := server.New(server.Options{
		Addr:          listenAddr,
		UnifiedClient: unifiedClient,
		Runner:        runner,
		Registry:      registry,
		Logger:        logger,
	})

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("failed to listen on port", "addr", listenAddr, "error", err)
		os.Exit(1)
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server running and ready for test requests", "dashboard_url", fmt.Sprintf("http://localhost%s", listenAddr))
		if err := srv.HTTPServer().Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// If startup test requested
	if *runOnStartup != "" {
		go func() {
			time.Sleep(500 * time.Millisecond) // brief settle
			logger.Info("executing startup tests", "suites", *runOnStartup)

			suites := []string{*runOnStartup}
			if *runOnStartup == "all" {
				suites = []string{"all"}
			}

			summary, err := runner.Run(context.Background(), testing.RunConfig{
				Suites:   suites,
				Protocol: "both",
			})

			if err != nil {
				logger.Error("startup test run failed", "error", err)
			} else {
				logger.Info("startup test run finished",
					"status", summary.Status,
					"passed", summary.Passed,
					"failed", summary.Failed,
					"total", summary.Total,
					"duration_ms", summary.Duration.Milliseconds(),
				)
			}

			if *autoExit {
				if summary != nil && summary.Status == testing.StatusPassed {
					logger.Info("auto-exit: all tests passed")
					os.Exit(0)
				} else {
					logger.Error("auto-exit: tests failed")
					os.Exit(1)
				}
			}
		}()
	}

	// 5. Graceful shutdown handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("shutting down test server", "signal", sig.String())
	case err := <-serverErr:
		logger.Error("http server unexpected failure", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.HTTPServer().Shutdown(ctx); err != nil {
		logger.Error("error during server shutdown", "error", err)
	}
	logger.Info("test server stopped cleanly")
}
