package storage

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default configuration values for the Redis connection pool.
const (
	DefaultPoolSize      = 10
	DefaultMinIdleConns  = 3
	DefaultDialTimeout   = 5 * time.Second
	DefaultReadTimeout   = 3 * time.Second
	DefaultWriteTimeout  = 3 * time.Second
	DefaultMaxRetries    = 3
	DefaultRetryDelay    = 100 * time.Millisecond
	DefaultMaxRetryDelay = 500 * time.Millisecond
)

// RedisConfig holds the configuration for the Redis storage backend.
type RedisConfig struct {
	// Addr is the Redis server address (host:port).
	Addr string
	// Password is the Redis password (empty for no auth).
	Password string
	// DB is the Redis database number.
	DB int

	// PoolSize is the maximum number of connections in the pool.
	PoolSize int
	// MinIdleConns is the minimum number of idle connections to maintain.
	MinIdleConns int

	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration
	// ReadTimeout is the timeout for read operations.
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration

	// MaxRetries is the maximum number of retries for failed commands.
	MaxRetries int
	// RetryDelay is the base delay between retries.
	RetryDelay time.Duration
	// MaxRetryDelay is the maximum delay between retries.
	MaxRetryDelay time.Duration
}

// DefaultRedisConfig returns a RedisConfig with sensible defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:          "localhost:6379",
		Password:      "",
		DB:            0,
		PoolSize:      DefaultPoolSize,
		MinIdleConns:  DefaultMinIdleConns,
		DialTimeout:   DefaultDialTimeout,
		ReadTimeout:   DefaultReadTimeout,
		WriteTimeout:  DefaultWriteTimeout,
		MaxRetries:    DefaultMaxRetries,
		RetryDelay:    DefaultRetryDelay,
		MaxRetryDelay: DefaultMaxRetryDelay,
	}
}

// RedisStorage implements the Storage interface using Redis.
type RedisStorage struct {
	client  *redis.Client
	scripts *scriptLoader
	mu      sync.RWMutex
	closed  bool
}

// NewRedisStorage creates a new Redis-backed storage instance.
// It validates the connection by sending a PING command.
func NewRedisStorage(ctx context.Context, cfg RedisConfig) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.RetryDelay,
		MaxRetryBackoff: cfg.MaxRetryDelay,
	})

	// Validate the connection.
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: failed to connect to %s: %w", cfg.Addr, err)
	}

	rs := &RedisStorage{
		client:  client,
		scripts: newScriptLoader(client),
	}

	// Pre-load Lua scripts into Redis script cache.
	if err := rs.scripts.LoadAll(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: failed to load Lua scripts: %w", err)
	}

	log.Printf("redis: connected to %s (pool_size=%d, min_idle=%d)",
		cfg.Addr, cfg.PoolSize, cfg.MinIdleConns)

	return rs, nil
}

// Increment atomically increments a rate limit counter for the given key.
// The counter is scoped to a time window using a key suffix.
func (rs *RedisStorage) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.closed {
		return 0, ErrStorageClosed
	}

	windowKey := windowedKey(key, window)
	windowMs := window.Milliseconds()

	result, err := rs.scripts.increment.Run(ctx, rs.client,
		[]string{windowKey},
		windowMs,
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis: increment failed for key %q: %w", key, err)
	}

	return result, nil
}

// GetCount returns the current request count for the given key within the window.
func (rs *RedisStorage) GetCount(ctx context.Context, key string, window time.Duration) (int64, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.closed {
		return 0, ErrStorageClosed
	}

	windowKey := windowedKey(key, window)

	count, err := rs.client.Get(ctx, windowKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis: get count failed for key %q: %w", key, err)
	}

	return count, nil
}

// Reset removes all rate limiting state associated with the given key.
// It deletes all keys matching the pattern "ratelimit:{key}:*".
func (rs *RedisStorage) Reset(ctx context.Context, key string) error {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.closed {
		return ErrStorageClosed
	}

	pattern := fmt.Sprintf("ratelimit:{%s}:*", key)

	var cursor uint64
	for {
		keys, nextCursor, err := rs.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("redis: scan failed for pattern %q: %w", pattern, err)
		}

		if len(keys) > 0 {
			if err := rs.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("redis: delete failed: %w", err)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// Ping checks connectivity to the Redis server.
func (rs *RedisStorage) Ping(ctx context.Context) error {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.closed {
		return ErrStorageClosed
	}

	if err := rs.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: ping failed: %w", err)
	}

	return nil
}

// Close gracefully shuts down the Redis connection.
func (rs *RedisStorage) Close() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.closed {
		return nil
	}

	rs.closed = true
	log.Println("redis: closing connection")

	return rs.client.Close()
}

// Client returns the underlying Redis client for advanced usage.
// Use with caution - prefer the Storage interface methods.
func (rs *RedisStorage) Client() *redis.Client {
	return rs.client
}

// PoolStats returns the current connection pool statistics.
func (rs *RedisStorage) PoolStats() *redis.PoolStats {
	return rs.client.PoolStats()
}

// windowedKey constructs a Redis key scoped to a specific time window.
// The window bucket is calculated from the current time, so keys naturally
// expire and rotate as time progresses.
func windowedKey(key string, window time.Duration) string {
	bucket := time.Now().UnixMilli() / window.Milliseconds()
	return fmt.Sprintf("ratelimit:{%s}:%d", key, bucket)
}
