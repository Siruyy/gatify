// Package limiter provides rate limiting functionality.
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/Siruyy/gatify/internal/storage"
)

// SlidingWindowStore defines storage capabilities required by the limiter.
type SlidingWindowStore interface {
	CheckSlidingWindow(ctx context.Context, key string, limit int64, window time.Duration) (storage.Result, error)
}

// Config controls limiter behavior.
type Config struct {
	Limit  int64
	Window time.Duration
}

// Limiter is a sliding-window rate limiter backed by a SlidingWindowStore.
type Limiter struct {
	store  SlidingWindowStore
	limit  int64
	window time.Duration
}

// New creates a limiter with the provided configuration.
func New(store SlidingWindowStore, cfg Config) (*Limiter, error) {
	if store == nil {
		return nil, fmt.Errorf("limiter: store is required")
	}
	if cfg.Limit <= 0 {
		return nil, fmt.Errorf("limiter: limit must be greater than 0")
	}
	if cfg.Window <= 0 {
		return nil, fmt.Errorf("limiter: window must be greater than 0")
	}

	return &Limiter{
		store:  store,
		limit:  cfg.Limit,
		window: cfg.Window,
	}, nil
}

// Allow checks whether a request for key should be permitted.
func (l *Limiter) Allow(ctx context.Context, key string) (storage.Result, error) {
	if key == "" {
		return storage.Result{}, fmt.Errorf("limiter: key is required")
	}

	return l.store.CheckSlidingWindow(ctx, key, l.limit, l.window)
}
