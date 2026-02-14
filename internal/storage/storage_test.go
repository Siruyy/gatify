package storage

import (
	"testing"
	"time"
)

func TestWindowedKey(t *testing.T) {
	key := "test-client"
	window := 60 * time.Second
	nowMs := int64(1700000000123)

	k1 := windowedKeyAt(key, window, nowMs)
	k2 := windowedKeyAt(key, window, nowMs)

	// Same key within the same window bucket should produce the same result.
	if k1 != k2 {
		t.Errorf("windowedKey produced different keys in the same window: %q vs %q", k1, k2)
	}

	// Key should contain the original key.
	if len(k1) == 0 {
		t.Error("windowedKey returned empty string")
	}

	// Different keys should produce different windowed keys.
	k3 := windowedKeyAt("other-client", window, nowMs)
	if k1 == k3 {
		t.Errorf("windowedKey produced same key for different clients: %q", k1)
	}
}

func TestDefaultRedisConfig(t *testing.T) {
	cfg := DefaultRedisConfig()

	if cfg.Addr != "localhost:6379" {
		t.Errorf("expected addr localhost:6379, got %s", cfg.Addr)
	}
	if cfg.PoolSize != DefaultPoolSize {
		t.Errorf("expected pool size %d, got %d", DefaultPoolSize, cfg.PoolSize)
	}
	if cfg.MinIdleConns != DefaultMinIdleConns {
		t.Errorf("expected min idle conns %d, got %d", DefaultMinIdleConns, cfg.MinIdleConns)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("expected max retries %d, got %d", DefaultMaxRetries, cfg.MaxRetries)
	}
	if cfg.DialTimeout != DefaultDialTimeout {
		t.Errorf("expected dial timeout %v, got %v", DefaultDialTimeout, cfg.DialTimeout)
	}
	if cfg.ReadTimeout != DefaultReadTimeout {
		t.Errorf("expected read timeout %v, got %v", DefaultReadTimeout, cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("expected write timeout %v, got %v", DefaultWriteTimeout, cfg.WriteTimeout)
	}
}

func TestStorageInterfaceCompliance(t *testing.T) {
	// Compile-time check that RedisStorage implements Storage.
	var _ Storage = (*RedisStorage)(nil)
}
