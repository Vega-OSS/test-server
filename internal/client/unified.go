package client

import (
	"context"
	"fmt"
	"strings"
)

// UnifiedClient holds both gRPC and REST clients for testing VegaDB.
type UnifiedClient struct {
	cfg  Config
	grpc *GRPCClient
	rest *RESTClient
}

// NewUnifiedClient creates both gRPC and REST connections to VegaDB.
func NewUnifiedClient(cfg Config) (*UnifiedClient, error) {
	grpcClient, err := NewGRPCClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("init grpc client: %w", err)
	}

	restClient, err := NewRESTClient(cfg)
	if err != nil {
		_ = grpcClient.Close()
		return nil, fmt.Errorf("init rest client: %w", err)
	}

	return &UnifiedClient{
		cfg:  cfg,
		grpc: grpcClient,
		rest: restClient,
	}, nil
}

// GRPC returns the gRPC client implementation.
func (u *UnifiedClient) GRPC() *GRPCClient {
	return u.grpc
}

// REST returns the REST client implementation.
func (u *UnifiedClient) REST() *RESTClient {
	return u.rest
}

// ClientFor returns the client for the specified protocol name ("grpc" or "rest").
func (u *UnifiedClient) ClientFor(proto string) (VegaClient, error) {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "grpc":
		return u.grpc, nil
	case "rest", "http":
		return u.rest, nil
	default:
		return nil, fmt.Errorf("unknown transport protocol: %s", proto)
	}
}

// HealthStatus holds health check results for both protocols.
type HealthStatus struct {
	GRPCConnected bool   `json:"grpc_connected"`
	GRPCError     string `json:"grpc_error,omitempty"`
	RESTConnected bool   `json:"rest_connected"`
	RESTError     string `json:"rest_error,omitempty"`
}

// CheckHealth verifies connectivity to VegaDB over both transports.
func (u *UnifiedClient) CheckHealth(ctx context.Context) HealthStatus {
	var hs HealthStatus

	if err := u.grpc.Health(ctx); err != nil {
		hs.GRPCConnected = false
		hs.GRPCError = err.Error()
	} else {
		hs.GRPCConnected = true
	}

	if err := u.rest.Health(ctx); err != nil {
		hs.RESTConnected = false
		hs.RESTError = err.Error()
	} else {
		hs.RESTConnected = true
	}

	return hs
}

// Close closes both gRPC and REST client connections.
func (u *UnifiedClient) Close() error {
	var errs []string
	if err := u.grpc.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("grpc: %v", err))
	}
	if err := u.rest.Close(); err != nil {
		errs = append(errs, fmt.Sprintf("rest: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("close clients: %s", strings.Join(errs, "; "))
	}
	return nil
}
