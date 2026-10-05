package client

import (
	"time"
)

// Config holds configuration parameters for connecting to a VegaDB instance.
type Config struct {
	GRPCAddr    string        `json:"grpc_addr"`
	RESTAddr    string        `json:"rest_addr"`
	GRPCPoolSize int          `json:"grpc_pool_size"`
	Timeout     time.Duration `json:"timeout"`
}

// DefaultConfig returns reasonable default configuration values.
func DefaultConfig() Config {
	return Config{
		GRPCAddr:     "127.0.0.1:50051",
		RESTAddr:     "http://127.0.0.1:8080",
		GRPCPoolSize: 4,
		Timeout:      5 * time.Second,
	}
}
