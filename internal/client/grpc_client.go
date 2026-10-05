package client

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/Vega-OSS/test-server/gen/vegapb"
)

// GRPCClient manages pooled gRPC connections to VegaDB.
type GRPCClient struct {
	addr    string
	timeout time.Duration
	conns   []*grpc.ClientConn
	clients []vegapb.VegaClient
	health  []healthpb.HealthClient
	next    atomic.Uint64
	closed  atomic.Bool
}

// NewGRPCClient initializes a pooled gRPC client.
func NewGRPCClient(cfg Config) (*GRPCClient, error) {
	addr := cfg.GRPCAddr
	if addr == "" {
		addr = "127.0.0.1:50051"
	}
	poolSize := cfg.GRPCPoolSize
	if poolSize <= 0 {
		poolSize = 4
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conns := make([]*grpc.ClientConn, poolSize)
	clients := make([]vegapb.VegaClient, poolSize)
	healthClients := make([]healthpb.HealthClient, poolSize)

	for i := 0; i < poolSize; i++ {
		conn, err := grpc.NewClient(addr, dialOpts...)
		if err != nil {
			for j := 0; j < i; j++ {
				_ = conns[j].Close()
			}
			return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
		}
		conns[i] = conn
		clients[i] = vegapb.NewVegaClient(conn)
		healthClients[i] = healthpb.NewHealthClient(conn)
	}

	return &GRPCClient{
		addr:    addr,
		timeout: timeout,
		conns:   conns,
		clients: clients,
		health:  healthClients,
	}, nil
}

func (c *GRPCClient) getClient() (vegapb.VegaClient, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	idx := c.next.Add(1) % uint64(len(c.clients))
	return c.clients[idx], nil
}

func (c *GRPCClient) Transport() string {
	return "grpc"
}

// Put sends a Put RPC.
func (c *GRPCClient) Put(ctx context.Context, key, val []byte) error {
	cl, err := c.getClient()
	if err != nil {
		return err
	}
	_, err = cl.Put(ctx, &vegapb.PutRequest{Key: key, Value: val})
	return mapGRPCError(err)
}

// Get sends a Get RPC.
func (c *GRPCClient) Get(ctx context.Context, key []byte) ([]byte, error) {
	cl, err := c.getClient()
	if err != nil {
		return nil, err
	}
	resp, err := cl.Get(ctx, &vegapb.GetRequest{Key: key})
	if err != nil {
		return nil, mapGRPCError(err)
	}
	return resp.GetValue(), nil
}

// Delete sends a Delete RPC.
func (c *GRPCClient) Delete(ctx context.Context, key []byte) error {
	cl, err := c.getClient()
	if err != nil {
		return err
	}
	_, err = cl.Delete(ctx, &vegapb.DeleteRequest{Key: key})
	return mapGRPCError(err)
}

// Scan streams keys and values in [start, end) up to limit.
func (c *GRPCClient) Scan(ctx context.Context, start, end []byte, limit uint32) ([]KV, error) {
	cl, err := c.getClient()
	if err != nil {
		return nil, err
	}
	stream, err := cl.Scan(ctx, &vegapb.ScanRequest{Start: start, End: end, Limit: limit})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	var results []KV
	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, mapGRPCError(err)
		}
		results = append(results, KV{
			Key:   item.GetKey(),
			Value: item.GetValue(),
		})
	}
	return results, nil
}

// BatchWrite sends an atomic batch of Puts and Deletes.
func (c *GRPCClient) BatchWrite(ctx context.Context, ops []BatchOp) error {
	cl, err := c.getClient()
	if err != nil {
		return err
	}

	pbOps := make([]*vegapb.BatchOp, len(ops))
	for i, op := range ops {
		var kind vegapb.BatchOp_Kind
		if op.Kind == BatchDelete {
			kind = vegapb.BatchOp_DELETE
		} else {
			kind = vegapb.BatchOp_PUT
		}
		pbOps[i] = &vegapb.BatchOp{
			Kind:  kind,
			Key:   op.Key,
			Value: op.Value,
		}
	}

	_, err = cl.BatchWrite(ctx, &vegapb.BatchRequest{Ops: pbOps})
	return mapGRPCError(err)
}

// Stats returns internal counters from the server.
func (c *GRPCClient) Stats(ctx context.Context) (map[string]int64, error) {
	cl, err := c.getClient()
	if err != nil {
		return nil, err
	}
	resp, err := cl.Stats(ctx, &vegapb.StatsRequest{})
	if err != nil {
		return nil, mapGRPCError(err)
	}
	return resp.GetCounters(), nil
}

// Health checks if VegaDB is reachable and serving.
func (c *GRPCClient) Health(ctx context.Context) error {
	if c.closed.Load() {
		return ErrClosed
	}
	idx := c.next.Add(1) % uint64(len(c.health))
	resp, err := c.health[idx].Check(ctx, &healthpb.HealthCheckRequest{Service: "vega.v1.Vega"})
	if err != nil {
		// Fallback to Stats call if standard health proto is not mounted
		_, statErr := c.Stats(ctx)
		if statErr == nil {
			return nil
		}
		return mapGRPCError(err)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("grpc health status not serving: %v", resp.GetStatus())
	}
	return nil
}

// Close closes all client connections.
func (c *GRPCClient) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	var firstErr error
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if s, ok := status.FromError(err); ok {
		if s.Code() == codes.NotFound {
			return ErrNotFound
		}
	}
	return err
}
