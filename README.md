# VegaDB Test Server

Autonomous test harness, assertion runner, and load testing server for [VegaDB](https://github.com/Vega-OSS/vega-db-2).

The Test Server validates running VegaDB instances across both **gRPC** and **HTTP/REST** interfaces, verifying correctness, crash-resilience, boundary cases, atomicity, high concurrency, and performance.

---

## Features

- **Dual-Protocol Testing:** Validates both gRPC (`:50051`) and HTTP/REST (`:8080`) transports, including cross-protocol validation (e.g. write via REST, read via gRPC).
- **Embedded Web Dashboard:** Single-page dashboard accessible at `http://localhost:8081` for one-click test execution, live status, pass/fail badges, and error diagnostics.
- **Comprehensive Test Suites:**
  - `smoke`: Baseline connectivity, health check, and basic CRUD lifecycle.
  - `crud`: Zero-byte values, large payloads (256KB+), binary keys with slashes/nulls, overwrite semantics, and 404 validation.
  - `scan`: Lexicographical range scans, boundary checks `[start, end)`, limit truncation, and empty ranges.
  - `batch`: Multi-operation atomic batches with mixed puts and deletes.
  - `concurrency`: Multi-worker parallel writes/reads, partitioned keyspaces, read-your-own-writes, and shared key contention.
  - `integrity`: Model-based randomized testing comparing database state against an in-memory oracle model.
  - `stress`: Burst load benchmark computing throughput (QPS) and latency percentiles (p50, p90, p95, p99).
- **Automation & CI Ready:** Run standalone as a daemon or in one-shot mode (`--run-on-startup=all --auto-exit`) in CI/CD pipelines.

---

## Quick Start

### 1. Build Binary

```bash
make build
```

### 2. Run Test Server

Ensure your VegaDB server is running (default gRPC on `127.0.0.1:50051`, REST on `http://127.0.0.1:8080`):

```bash
./bin/testserver --port=:8081 --vega-grpc=127.0.0.1:50051 --vega-http=http://127.0.0.1:8080
```

### 3. Open Web Dashboard

Navigate to `http://localhost:8081` in your browser to launch test suites interactively.

---

## REST API Reference

| Endpoint | Method | Description |
|---|---|---|
| `/healthz` | `GET` | Test server liveness |
| `/vega/health` | `GET` | Connectivity status for VegaDB gRPC and REST |
| `/api/v1/test/suites` | `GET` | List all registered test suites |
| `/api/v1/test/run` | `POST` | Trigger test suite execution (JSON payload) |
| `/api/v1/test/history` | `GET` | List past test execution summaries |
| `/api/v1/test/history/{id}` | `GET` | Retrieve full details and logs of a specific test run |
| `/api/v1/test/cancel/{id}` | `POST` | Cancel an active in-progress test run |
| `/` | `GET` | Interactive browser dashboard |

### Example API Requests

#### Check VegaDB Connectivity
```bash
curl -s http://localhost:8081/vega/health | jq
```

#### Run All Test Suites over Both Protocols
```bash
curl -s -X POST http://localhost:8081/api/v1/test/run \
  -H "Content-Type: application/json" \
  -d '{"suites": ["all"], "protocol": "both"}' | jq
```

#### Run Specific Suites (e.g. Smoke & CRUD over gRPC)
```bash
curl -s -X POST http://localhost:8081/api/v1/test/run \
  -H "Content-Type: application/json" \
  -d '{"suites": ["smoke", "crud"], "protocol": "grpc"}' | jq
```

#### Fetch Test Run History
```bash
curl -s http://localhost:8081/api/v1/test/history | jq
```

---

## CLI Configuration Flags

| Flag | Env Variable | Default | Description |
|---|---|---|---|
| `--port` | `PORT` | `:8081` | Test server HTTP listen address |
| `--vega-grpc` | `VEGA_GRPC_ADDR` | `127.0.0.1:50051` | VegaDB gRPC endpoint |
| `--vega-http` | `VEGA_HTTP_ADDR` | `http://127.0.0.1:8080` | VegaDB REST URL |
| `--grpc-pool` | `GRPC_POOL_SIZE` | `4` | Pooled gRPC connections |
| `--run-on-startup` | `RUN_ON_STARTUP` | `""` | Run tests on startup (`smoke`, `all`, etc.) |
| `--auto-exit` | `AUTO_EXIT` | `false` | Exit process after startup tests (exit 0/1) |
| `--log-level` | `LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |

---

## Running with Docker Compose

Run both VegaDB and the Test Server together:

```bash
docker compose up -d --build
```

Then visit `http://localhost:8081`.

---

## Testing & Quality

Run unit and race-condition tests:

```bash
make check
```
