package client

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.GRPCAddr != "127.0.0.1:50051" {
		t.Errorf("expected default gRPC addr 127.0.0.1:50051, got %s", cfg.GRPCAddr)
	}
	if cfg.RESTAddr != "http://127.0.0.1:8080" {
		t.Errorf("expected default REST addr http://127.0.0.1:8080, got %s", cfg.RESTAddr)
	}
	if cfg.GRPCPoolSize != 4 {
		t.Errorf("expected default pool size 4, got %d", cfg.GRPCPoolSize)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected default timeout 5s, got %v", cfg.Timeout)
	}
}

func TestEncodeKey(t *testing.T) {
	tests := []struct {
		name       string
		key        []byte
		wantB64    bool
		wantString string
	}{
		{
			name:       "simple ascii",
			key:        []byte("user123"),
			wantB64:    false,
			wantString: "user123",
		},
		{
			name:       "key with slash",
			key:        []byte("user/profile"),
			wantB64:    true,
			wantString: base64.RawURLEncoding.EncodeToString([]byte("user/profile")),
		},
		{
			name:       "binary key with nulls",
			key:        []byte{0x00, 0x01, 0xFF},
			wantB64:    true,
			wantString: base64.RawURLEncoding.EncodeToString([]byte{0x00, 0x01, 0xFF}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, isB64 := encodeKey(tt.key)
			if isB64 != tt.wantB64 {
				t.Errorf("encodeKey() isB64 = %v, want %v", isB64, tt.wantB64)
			}
			if path != tt.wantString {
				t.Errorf("encodeKey() path = %v, want %v", path, tt.wantString)
			}
		})
	}
}
