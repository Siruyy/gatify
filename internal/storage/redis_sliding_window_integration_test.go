//go:build integration

package storage

import (
	"context"
	"testing"
	"time"
)

func TestRedisStorage_CheckSlidingWindow_AllowsUntilLimit(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	key := "test:sliding:allow:" + t.Name()
	window := 2 * time.Second
	limit := int64(3)

	_ = rs.Reset(ctx, key)

	for i := int64(1); i <= limit; i++ {
		result, err := rs.CheckSlidingWindow(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("CheckSlidingWindow failed at iteration %d: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("expected allowed=true at iteration %d, got false", i)
		}
		if result.Limit != limit {
			t.Fatalf("expected limit %d, got %d", limit, result.Limit)
		}
		if result.Count <= 0 {
			t.Fatalf("expected positive count, got %d", result.Count)
		}
	}

	result, err := rs.CheckSlidingWindow(ctx, key, limit, window)
	if err != nil {
		t.Fatalf("CheckSlidingWindow failed after limit exceeded: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected allowed=false after exceeding limit, got true (count=%d)", result.Count)
	}
}

func TestRedisStorage_CheckSlidingWindow_Validation(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	if _, err := rs.CheckSlidingWindow(ctx, "key", 0, time.Second); err == nil {
		t.Fatal("expected error for zero limit")
	}
	if _, err := rs.CheckSlidingWindow(ctx, "key", 1, 0); err == nil {
		t.Fatal("expected error for zero window")
	}
}

func TestRedisStorage_CheckSlidingWindow_AfterClose(t *testing.T) {
	rs := newTestStorage(t)
	ctx := context.Background()

	if err := rs.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := rs.CheckSlidingWindow(ctx, "key", 10, time.Second); err != ErrStorageClosed {
		t.Fatalf("expected ErrStorageClosed, got %v", err)
	}
}
