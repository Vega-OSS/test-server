package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RESTClient manages HTTP/REST requests to VegaDB.
type RESTClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRESTClient initializes a new REST client.
func NewRESTClient(cfg Config) (*RESTClient, error) {
	baseURL := strings.TrimRight(cfg.RESTAddr, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &RESTClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

func (c *RESTClient) Transport() string {
	return "rest"
}

// encodeKey returns the URL path segment and whether base64url encoding is needed.
func encodeKey(key []byte) (string, bool) {
	str := string(key)
	needsBase64 := false
	for _, b := range key {
		if b < 0x20 || b > 0x7E || b == '/' || b == '?' || b == '#' {
			needsBase64 = true
			break
		}
	}
	if needsBase64 {
		return base64.RawURLEncoding.EncodeToString(key), true
	}
	return url.PathEscape(str), false
}

// Put stores a key-value pair via PUT /v1/kv/{key}.
func (c *RESTClient) Put(ctx context.Context, key, val []byte) error {
	keyPath, isB64 := encodeKey(key)
	reqURL := fmt.Sprintf("%s/v1/kv/%s", c.baseURL, keyPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(val))
	if err != nil {
		return fmt.Errorf("create put request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if isB64 {
		req.Header.Set("X-Key-Encoding", "base64url")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("exec put request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("put failed with status %d: %s", resp.StatusCode, string(body))
}

// Get retrieves a key's value via GET /v1/kv/{key}.
func (c *RESTClient) Get(ctx context.Context, key []byte) ([]byte, error) {
	keyPath, isB64 := encodeKey(key)
	reqURL := fmt.Sprintf("%s/v1/kv/%s", c.baseURL, keyPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create get request: %w", err)
	}
	if isB64 {
		req.Header.Set("X-Key-Encoding", "base64url")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exec get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get failed with status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// Delete removes a key via DELETE /v1/kv/{key}.
func (c *RESTClient) Delete(ctx context.Context, key []byte) error {
	keyPath, isB64 := encodeKey(key)
	reqURL := fmt.Sprintf("%s/v1/kv/%s", c.baseURL, keyPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	if isB64 {
		req.Header.Set("X-Key-Encoding", "base64url")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("exec delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
}

type restScanItem struct {
	Key   string `json:"key"`
	Value string `json:"value"` // base64
}

// Scan queries a key range via GET /v1/scan and decodes NDJSON.
func (c *RESTClient) Scan(ctx context.Context, start, end []byte, limit uint32) ([]KV, error) {
	params := url.Values{}
	if len(start) > 0 {
		params.Set("start", string(start))
	}
	if len(end) > 0 {
		params.Set("end", string(end))
	}
	if limit > 0 {
		params.Set("limit", strconv.FormatUint(uint64(limit), 10))
	}

	reqURL := fmt.Sprintf("%s/v1/scan?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create scan request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exec scan request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scan failed with status %d: %s", resp.StatusCode, string(body))
	}

	var results []KV
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var item restScanItem
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, fmt.Errorf("decode scan item: %w", err)
		}

		valBytes, err := base64.StdEncoding.DecodeString(item.Value)
		if err != nil {
			return nil, fmt.Errorf("decode scan base64 value: %w", err)
		}

		results = append(results, KV{
			Key:   []byte(item.Key),
			Value: valBytes,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan stream error: %w", err)
	}

	return results, nil
}

type restBatchOp struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"` // base64
}

type restBatchRequest struct {
	Ops []restBatchOp `json:"ops"`
}

// BatchWrite sends batch operations via POST /v1/batch.
func (c *RESTClient) BatchWrite(ctx context.Context, ops []BatchOp) error {
	var req restBatchRequest
	req.Ops = make([]restBatchOp, len(ops))

	for i, op := range ops {
		if op.Kind == BatchDelete {
			req.Ops[i] = restBatchOp{
				Op:  "delete",
				Key: string(op.Key),
			}
		} else {
			req.Ops[i] = restBatchOp{
				Op:    "put",
				Key:   string(op.Key),
				Value: base64.StdEncoding.EncodeToString(op.Value),
			}
		}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode batch request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/v1/batch", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create batch request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("exec batch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("batch failed with status %d: %s", resp.StatusCode, string(body))
}

// Stats returns internal counters via GET /v1/stats.
func (c *RESTClient) Stats(ctx context.Context) (map[string]int64, error) {
	reqURL := fmt.Sprintf("%s/v1/stats", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create stats request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exec stats request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stats failed with status %d: %s", resp.StatusCode, string(body))
	}

	var stats map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode stats json: %w", err)
	}
	return stats, nil
}

// Health checks if VegaDB is live and ready via GET /readyz or /healthz.
func (c *RESTClient) Health(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/readyz", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("exec readyz request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz returned status %d", resp.StatusCode)
	}
	return nil
}

// Close closes any idle HTTP transport connections.
func (c *RESTClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
