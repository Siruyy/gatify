//go:build integration

package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// postgresURL returns the PostgreSQL connection URL for integration tests.
// It defaults to the docker-compose timescaledb instance but can be overridden via DATABASE_URL.
func postgresURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable"
	}
	return url
}

// setupTestDB creates a test database connection and sets up the test schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", postgresURL(t))
	if err != nil {
		t.Skipf("Failed to connect to PostgreSQL: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	// Create test table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS rate_limit_events (
			id SERIAL PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL,
			client_id TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			allowed BOOLEAN NOT NULL,
			rule_id TEXT,
			limit_value BIGINT,
			remaining BIGINT,
			response_ms BIGINT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Clean up previous test data
	_, err = db.ExecContext(ctx, "TRUNCATE rate_limit_events")
	if err != nil {
		t.Fatalf("Failed to truncate test table: %v", err)
	}

	t.Cleanup(func() {
		// Clean up after test
		_, _ = db.Exec("TRUNCATE rate_limit_events")
		db.Close()
	})

	return db
}

func TestLogger_FlushIntegration(t *testing.T) {
	db := setupTestDB(t)

	// Create logger with small batch size for faster testing
	logger, err := New(Config{
		DB:            db,
		BufferSize:    10,
		BatchSize:     5,
		FlushInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log some events
	events := []Event{
		{
			Timestamp:  time.Now(),
			ClientID:   "192.168.1.1",
			Method:     "GET",
			Path:       "/api/users",
			Allowed:    true,
			RuleID:     "rule-1",
			Limit:      100,
			Remaining:  95,
			ResponseMS: 45,
		},
		{
			Timestamp:  time.Now(),
			ClientID:   "192.168.1.2",
			Method:     "POST",
			Path:       "/api/users",
			Allowed:    false,
			RuleID:     "rule-1",
			Limit:      100,
			Remaining:  0,
			ResponseMS: 12,
		},
		{
			Timestamp:  time.Now(),
			ClientID:   "10.0.0.1",
			Method:     "GET",
			Path:       "/api/products",
			Allowed:    true,
			RuleID:     "rule-2",
			Limit:      50,
			Remaining:  30,
			ResponseMS: 78,
		},
	}

	for _, event := range events {
		logger.Log(event)
	}

	// Wait for flush
	time.Sleep(500 * time.Millisecond)

	// Close logger to ensure all events are flushed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := logger.Close(ctx); err != nil {
		t.Fatalf("Failed to close logger: %v", err)
	}

	// Verify events were written to database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rate_limit_events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != len(events) {
		t.Errorf("Expected %d events in database, got %d", len(events), count)
	}

	// Verify event details
	rows, err := db.Query(`
		SELECT client_id, method, path, allowed, rule_id, limit_value, remaining, response_ms
		FROM rate_limit_events
		ORDER BY id
	`)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var clientID, method, path, ruleID string
		var allowed bool
		var limit, remaining, responseMS int64

		err := rows.Scan(&clientID, &method, &path, &allowed, &ruleID, &limit, &remaining, &responseMS)
		if err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		expected := events[i]
		if clientID != expected.ClientID {
			t.Errorf("Event %d: expected ClientID %s, got %s", i, expected.ClientID, clientID)
		}
		if method != expected.Method {
			t.Errorf("Event %d: expected Method %s, got %s", i, expected.Method, method)
		}
		if path != expected.Path {
			t.Errorf("Event %d: expected Path %s, got %s", i, expected.Path, path)
		}
		if allowed != expected.Allowed {
			t.Errorf("Event %d: expected Allowed %v, got %v", i, expected.Allowed, allowed)
		}

		i++
	}
}

func TestLogger_BatchFlushIntegration(t *testing.T) {
	db := setupTestDB(t)

	// Create logger with batch size of 10
	logger, err := New(Config{
		DB:            db,
		BufferSize:    100,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour, // Long interval to test batch-based flushing
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log exactly 10 events to trigger batch flush
	for i := 0; i < 10; i++ {
		logger.Log(Event{
			Timestamp:  time.Now(),
			ClientID:   fmt.Sprintf("client-%d", i),
			Method:     "GET",
			Path:       "/api/test",
			Allowed:    true,
			RuleID:     "rule-1",
			Limit:      100,
			Remaining:  90,
			ResponseMS: 50,
		})
	}

	// Give time for batch to flush
	time.Sleep(500 * time.Millisecond)

	// Verify events were written
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rate_limit_events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 10 {
		t.Errorf("Expected 10 events after batch flush, got %d", count)
	}

	// Close logger
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = logger.Close(ctx)
}

func TestLogger_GracefulShutdownIntegration(t *testing.T) {
	db := setupTestDB(t)

	// Create logger with long flush interval
	logger, err := New(Config{
		DB:            db,
		BufferSize:    100,
		BatchSize:     100,
		FlushInterval: 1 * time.Hour, // Very long to ensure manual flush on close
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log events that won't be auto-flushed
	for i := 0; i < 5; i++ {
		logger.Log(Event{
			Timestamp: time.Now(),
			ClientID:  fmt.Sprintf("client-%d", i),
			Method:    "GET",
			Path:      "/api/shutdown-test",
			Allowed:   true,
			RuleID:    "rule-1",
			Limit:     100,
			Remaining: 95,
		})
	}

	// Close logger immediately (should flush pending events)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := logger.Close(ctx); err != nil {
		t.Fatalf("Failed to close logger: %v", err)
	}

	// Verify all events were persisted despite no automatic flush
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rate_limit_events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 events after graceful shutdown, got %d", count)
	}

	// Check stats
	logged, dropped := logger.Stats()
	if logged != 5 {
		t.Errorf("Expected 5 logged events, got %d", logged)
	}
	if dropped != 0 {
		t.Errorf("Expected 0 dropped events, got %d", dropped)
	}
}

func TestLogger_HighLoadIntegration(t *testing.T) {
	db := setupTestDB(t)

	// Create logger
	logger, err := New(Config{
		DB:            db,
		BufferSize:    1000,
		BatchSize:     100,
		FlushInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Log many events quickly
	const numEvents = 500
	for i := 0; i < numEvents; i++ {
		logger.Log(Event{
			Timestamp:  time.Now(),
			ClientID:   fmt.Sprintf("client-%d", i%50),
			Method:     "GET",
			Path:       "/api/load-test",
			Allowed:    i%10 != 0, // 10% blocked
			RuleID:     "rule-high-load",
			Limit:      1000,
			Remaining:  int64(1000 - (i % 1000)),
			ResponseMS: int64(20 + (i % 100)),
		})
	}

	// Close and flush
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := logger.Close(ctx); err != nil {
		t.Fatalf("Failed to close logger: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rate_limit_events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	// Allow for some dropped events under high load
	if count < numEvents-10 {
		t.Errorf("Expected at least %d events, got %d", numEvents-10, count)
	}

	logged, dropped := logger.Stats()
	t.Logf("Logged: %d, Dropped: %d", logged, dropped)

	if logged+dropped != numEvents {
		t.Errorf("logged + dropped should equal total events: %d + %d != %d", logged, dropped, numEvents)
	}
}
