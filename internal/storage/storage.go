// Package storage provides data storage interfaces and implementations
// for the Gatify rate limiting backend.
package storage

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrStorageClosed is returned when an operation is attempted on a closed storage.
	ErrStorageClosed = errors.New("storage: connection closed")

	// ErrKeyNotFound is returned when a key does not exist.
	ErrKeyNotFound = errors.New("storage: key not found")
)

// Result holds the outcome of a rate limit check against storage.
type Result struct {
	// Count is the current request count in the window.
	Count int64
	// Limit is the maximum allowed requests in the window.
	Limit int64
	// Remaining is how many requests are still allowed.
	Remaining int64
	// ResetAt is the time when the current window expires.
	ResetAt time.Time
	// Allowed indicates whether the request should be permitted.
	Allowed bool
}

// Storage defines the interface for rate limiting state backends.
// All methods must be safe for concurrent use.
type Storage interface {
	// Increment atomically increments the counter for a given key within
	// the specified window duration. It returns the count after incrementing.
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)

	// GetCount returns the current count for a key within the specified window.
	GetCount(ctx context.Context, key string, window time.Duration) (int64, error)

	// CheckSlidingWindow performs a sliding-window rate limit check and increments
	// the current window counter if allowed.
	CheckSlidingWindow(ctx context.Context, key string, limit int64, window time.Duration) (Result, error)

	// Reset removes all rate limiting state for the given key.
	Reset(ctx context.Context, key string) error

	// Ping checks the health of the storage backend.
	Ping(ctx context.Context) error

	// Close gracefully shuts down the storage connection.
	Close() error
}
