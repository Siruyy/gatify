package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Lua scripts for atomic rate limiting operations.
//
// Using Lua scripts ensures that multi-step operations (INCR + PEXPIRE)
// execute atomically on the Redis server, preventing race conditions
// between concurrent requests.

// luaIncrement atomically increments a key and sets its expiry.
// KEYS[1] = the rate limit key
// ARGV[1] = window duration in milliseconds
//
// Returns the count after incrementing.
const luaIncrement = `
local key = KEYS[1]
local window_ms = tonumber(ARGV[1])

local current = redis.call("INCR", key)

-- Only set expiry on first increment (when count becomes 1)
-- to avoid resetting the TTL on subsequent increments.
if current == 1 then
    redis.call("PEXPIRE", key, window_ms)
end

return current
`

// luaSlidingWindowCheck performs a sliding window rate limit check.
// KEYS[1] = current window key
// KEYS[2] = previous window key
// ARGV[1] = window duration in milliseconds
// ARGV[2] = elapsed time in the current window (milliseconds)
// ARGV[3] = rate limit
//
// Returns: {allowed (0/1), current_count, previous_count}
//
// This script is pre-loaded for GAT-6 (sliding window counter algorithm)
// but the actual sliding window logic will be implemented there.
const luaSlidingWindowCheck = `
local current_key = KEYS[1]
local previous_key = KEYS[2]
local window_ms = tonumber(ARGV[1])
local elapsed_ms = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

local current_count = tonumber(redis.call("GET", current_key) or "0")
local previous_count = tonumber(redis.call("GET", previous_key) or "0")

-- Weighted count: previous window's proportion + current window's count
local weight = 1 - (elapsed_ms / window_ms)
local estimated = math.floor(previous_count * weight) + current_count

if estimated >= limit then
    return {0, current_count, previous_count}
end

-- Increment current window
local new_count = redis.call("INCR", current_key)
if new_count == 1 then
    redis.call("PEXPIRE", current_key, window_ms * 2)
end

return {1, new_count, previous_count}
`

// scriptLoader manages the lifecycle of Lua scripts in Redis.
// Scripts are loaded once via SCRIPT LOAD and then executed by SHA,
// which reduces bandwidth and parsing overhead on repeated calls.
type scriptLoader struct {
	client *redis.Client

	increment          *redis.Script
	slidingWindowCheck *redis.Script
}

// newScriptLoader creates a new script loader with all scripts registered.
func newScriptLoader(client *redis.Client) *scriptLoader {
	return &scriptLoader{
		client:             client,
		increment:          redis.NewScript(luaIncrement),
		slidingWindowCheck: redis.NewScript(luaSlidingWindowCheck),
	}
}

// LoadAll pre-loads all Lua scripts into the Redis script cache.
// This should be called once during initialization. The go-redis library
// handles transparent reloading if scripts are evicted from the cache.
func (sl *scriptLoader) LoadAll(ctx context.Context) error {
	scripts := map[string]*redis.Script{
		"increment":            sl.increment,
		"sliding_window_check": sl.slidingWindowCheck,
	}

	for name, script := range scripts {
		if err := script.Load(ctx, sl.client).Err(); err != nil {
			return fmt.Errorf("failed to load script %q: %w", name, err)
		}
	}

	return nil
}
