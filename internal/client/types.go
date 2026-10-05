package client

import (
	"context"
	"errors"
)

// Standard sentinel errors.
var (
	ErrNotFound = errors.New("key not found")
	ErrClosed   = errors.New("client is closed")
)

// KV represents a key-value pair.
type KV struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

// BatchOpKind represents the type of a batch operation.
type BatchOpKind int

const (
	BatchPut BatchOpKind = iota
	BatchDelete
)

// BatchOp represents a single operation in an atomic batch.
type BatchOp struct {
	Kind  BatchOpKind `json:"kind"`
	Key   []byte      `json:"key"`
	Value []byte      `json:"value,omitempty"`
}

// VegaClient is the unified interface for interacting with VegaDB.
type VegaClient interface {
	Put(ctx context.Context, key, val []byte) error
	Get(ctx context.Context, key []byte) ([]byte, error)
	Delete(ctx context.Context, key []byte) error
	Scan(ctx context.Context, start, end []byte, limit uint32) ([]KV, error)
	BatchWrite(ctx context.Context, ops []BatchOp) error
	Stats(ctx context.Context) (map[string]int64, error)
	Health(ctx context.Context) error
	Close() error
	Transport() string
}
