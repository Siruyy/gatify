//go:build integration

package storage

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// redisAddr returns the Redis address for integration tests.
// It defaults to localhost:6379 but can be overridden via REDIS_ADDR.
func redisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return addr
}

// newTestStorage creates a RedisStorage instance for testing.
// It skips the test if Redis is not available.
func newTestStorage(t *testing.T) *RedisStorage {
	t.Helper()

	cfg := DefaultRedisConfig()
	cfg.Addr = redisAddr(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rs, err := NewRedisStorage(ctx, cfg)
	if err != nil {
		t.Skipf("Redis not available at %s: %v", cfg.Addr, err)
	}

	t.Cleanup(func() {
		_ = rs.Close()
	})

	return rs
}

func TestRedisStorage_Ping(t *testing.T) {
	rs := newTestStorage(t)

	ctx := context.Background()
	if err := rs.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestRedisStorage_Increment(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	key := "test:increment:" + t.Name()
	window := 10 * time.Second

	// Clean up before test.
	_ = rs.Reset(ctx, key)

	// First increment should return 1.
	count, err := rs.Increment(ctx, key, window)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	// Second increment should return 2.
	count, err = rs.Increment(ctx, key, window)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	// Third increment should return 3.
	count, err = rs.Increment(ctx, key, window)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestRedisStorage_GetCount(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	key := "test:getcount:" + t.Name()
	window := 10 * time.Second

	// Clean up before test.
	_ = rs.Reset(ctx, key)

	// Count for nonexistent key should be 0.
	count, err := rs.GetCount(ctx, key, window)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	// Increment then check count.
	_, err = rs.Increment(ctx, key, window)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	count, err = rs.GetCount(ctx, key, window)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestRedisStorage_Reset(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	key := "test:reset:" + t.Name()
	window := 10 * time.Second

	// Increment a few times.
	for i := 0; i < 5; i++ {
		_, err := rs.Increment(ctx, key, window)
		if err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
	}

	// Reset should succeed.
	if err := rs.Reset(ctx, key); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Count after reset should be 0.
	count, err := rs.GetCount(ctx, key, window)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 after reset, got %d", count)
	}
}

func TestRedisStorage_Close(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	// Close the storage.
	if err := rs.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail with ErrStorageClosed.
	_, err := rs.Increment(ctx, "test:closed", 10*time.Second)
	if err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed, got %v", err)
	}

	if err := rs.Ping(ctx); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed from Ping, got %v", err)
	}

	// Double close should not error.
	if err := rs.Close(); err != nil {
		t.Errorf("double Close should not error, got %v", err)
	}
}

func TestRedisStorage_ConcurrentIncrements(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	key := "test:concurrent:" + t.Name()
	window := 10 * time.Second

	// Clean up before test.
	_ = rs.Reset(ctx, key)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := rs.Increment(ctx, key, window)
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent increment error: %v", err)
	}

	// Final count should equal the number of goroutines.
	count, err := rs.GetCount(ctx, key, window)
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != goroutines {
		t.Errorf("expected count %d, got %d", goroutines, count)
	}
}

func TestRedisStorage_PoolStats(t *testing.T) {
	rs := newTestStorage(t)

	stats := rs.PoolStats()
	if stats == nil {
		t.Fatal("PoolStats returned nil")
	}

	// After a successful connection, we should have at least some pool activity.
	t.Logf("Pool stats: hits=%d misses=%d timeouts=%d total=%d idle=%d stale=%d",
		stats.Hits, stats.Misses, stats.Timeouts, stats.TotalConns, stats.IdleConns, stats.StaleConns)
}

func TestRedisStorage_ConnectionFailure(t *testing.T) {
	cfg := DefaultRedisConfig()
	cfg.Addr = "localhost:59999" // Non-existent port.
	cfg.DialTimeout = 1 * time.Second
	cfg.MaxRetries = 0

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := NewRedisStorage(ctx, cfg)
	if err == nil {
		t.Fatal("expected error connecting to non-existent Redis, got nil")
	}
}
