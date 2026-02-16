//go:build integration

package migrations

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
)

// postgresURL returns the PostgreSQL connection URL for integration tests.
// It defaults to localhost but can be overridden via DATABASE_URL.
func postgresURL(t *testing.T) string {
	t.Helper()
	rawURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if rawURL != "" {
		return rawURL
	}

	host := envOrDefault("POSTGRES_HOST", "localhost")
	port := envOrDefault("POSTGRES_PORT", "5432")
	dbName := envOrDefault("POSTGRES_DB", "gatify")
	user := envOrDefault("POSTGRES_USER", "gatify")
	password := envOrDefault("POSTGRES_PASSWORD", "gatify_dev_password")
	sslMode := envOrDefault("POSTGRES_SSLMODE", "disable")

	builtURL := &url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + dbName,
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
		User:     url.UserPassword(user, password),
	}

	return builtURL.String()
}

// createTestDatabase creates a test database for migration tests.
func createTestDatabase(t *testing.T) string {
	t.Helper()

	baseURL := postgresURL(t)
	testDBName := fmt.Sprintf("gatify_test_migrations_%d", os.Getpid())

	// Connect to default database to create test database
	db, err := sql.Open("postgres", baseURL)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	// Drop test database if it exists
	_, _ = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))

	// Create test database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	if err != nil {
		t.Skipf("Cannot create test database: %v", err)
	}

	// Cleanup: drop test database after test
	t.Cleanup(func() {
		db, err := sql.Open("postgres", baseURL)
		if err != nil {
			return
		}
		defer db.Close()
		_, _ = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	})

	// Return URL for test database
	testURL, err := withDatabaseName(baseURL, testDBName)
	if err != nil {
		t.Fatalf("Failed to construct test database URL: %v", err)
	}

	return testURL
}

func withDatabaseName(rawURL, dbName string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid database URL %q: %w", rawURL, err)
	}

	parsed.Path = "/" + dbName
	return parsed.String(), nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func TestNewRunner(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	if runner.db == nil {
		t.Error("Expected runner.db to be non-nil")
	}
	if runner.migrate == nil {
		t.Error("Expected runner.migrate to be non-nil")
	}
}

func TestNewRunner_EmptyURL(t *testing.T) {
	_, err := NewRunner("")
	if err == nil {
		t.Error("Expected error for empty database URL")
	}
}

func TestRunner_Version_BeforeMigration(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	version, dirty, err := runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}

	if version != 0 {
		t.Errorf("Expected version 0 before migration, got %d", version)
	}
	if dirty {
		t.Error("Expected clean state before migration")
	}
}

func TestRunner_Up(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	// Run migrations up
	if err := runner.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	// Check version after migration
	version, dirty, err := runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}

	if version == 0 {
		t.Error("Expected version > 0 after migration")
	}
	if dirty {
		t.Error("Expected clean state after successful migration")
	}

	// Verify that tables were created
	var exists bool
	err = runner.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'rules')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check for rules table: %v", err)
	}
	if !exists {
		t.Error("Expected rules table to exist after migration")
	}

	err = runner.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'rate_limit_events')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check for rate_limit_events table: %v", err)
	}
	if !exists {
		t.Error("Expected rate_limit_events table to exist after migration")
	}

	// Verify that seed data was inserted
	var count int
	err = runner.db.QueryRow("SELECT COUNT(*) FROM rules WHERE id = 'default-global'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check seed data: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 seed rule, got %d", count)
	}
}

func TestRunner_Down(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	// Run migrations up first
	if err := runner.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	// Run migrations down
	if err := runner.Down(); err != nil {
		t.Fatalf("Down() failed: %v", err)
	}

	// Check version after rollback
	version, dirty, err := runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}

	if version != 0 {
		t.Errorf("Expected version 0 after rollback, got %d", version)
	}
	if dirty {
		t.Error("Expected clean state after successful rollback")
	}

	// Verify that tables were dropped
	var exists bool
	err = runner.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'rules')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check for rules table: %v", err)
	}
	if exists {
		t.Error("Expected rules table to be dropped after rollback")
	}

	err = runner.db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'rate_limit_events')").Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check for rate_limit_events table: %v", err)
	}
	if exists {
		t.Error("Expected rate_limit_events table to be dropped after rollback")
	}
}

func TestRunner_Steps(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	// Apply first migration (step +1)
	if err := runner.Steps(1); err != nil {
		t.Fatalf("Steps(1) failed: %v", err)
	}

	version, _, err := runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1 after 1 step, got %d", version)
	}

	// Apply second migration (step +1)
	if err := runner.Steps(1); err != nil {
		t.Fatalf("Steps(1) failed: %v", err)
	}

	version, _, err = runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 2 {
		t.Errorf("Expected version 2 after 2 steps, got %d", version)
	}

	// Rollback one migration (step -1)
	if err := runner.Steps(-1); err != nil {
		t.Fatalf("Steps(-1) failed: %v", err)
	}

	version, _, err = runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1 after rollback, got %d", version)
	}
}

func TestRunner_MigrateTo(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	// Migrate to version 1
	if err := runner.MigrateTo(1); err != nil {
		t.Fatalf("MigrateTo(1) failed: %v", err)
	}

	version, _, err := runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}

	// Migrate to version 2
	if err := runner.MigrateTo(2); err != nil {
		t.Fatalf("MigrateTo(2) failed: %v", err)
	}

	version, _, err = runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 2 {
		t.Errorf("Expected version 2, got %d", version)
	}

	// Migrate back to version 1
	if err := runner.MigrateTo(1); err != nil {
		t.Fatalf("MigrateTo(1) failed: %v", err)
	}

	version, _, err = runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1 after going back, got %d", version)
	}
}

func TestRunner_Force(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	// Apply migrations first
	if err := runner.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	// Force version to 1
	if err := runner.Force(1); err != nil {
		t.Fatalf("Force(1) failed: %v", err)
	}

	version, dirty, err := runner.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1 after force, got %d", version)
	}
	if dirty {
		t.Error("Expected clean state after force")
	}
}

func TestRunner_RulesTableSchema(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	if err := runner.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	// Verify rules table has all expected columns
	expectedColumns := []string{
		"id", "name", "pattern", "methods", "priority",
		"limit_value", "window_seconds", "identify_by", "header_name",
		"enabled", "description", "created_at", "updated_at",
	}

	for _, col := range expectedColumns {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'rules' AND column_name = $1
			)
		`
		err = runner.db.QueryRow(query, col).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check for column %s: %v", col, err)
		}
		if !exists {
			t.Errorf("Expected column %s to exist in rules table", col)
		}
	}
}

func TestRunner_RateLimitEventsIndexes(t *testing.T) {
	testURL := createTestDatabase(t)

	runner, err := NewRunner(testURL)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}
	defer runner.Close()

	if err := runner.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	// Verify expected indexes exist
	expectedIndexes := []string{
		"idx_rate_limit_events_timestamp_desc",
		"idx_rate_limit_events_rule_time",
		"idx_rate_limit_events_path_time",
		"idx_rate_limit_events_allowed_time",
		"idx_rate_limit_events_client_time",
	}

	for _, idx := range expectedIndexes {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT FROM pg_indexes 
				WHERE tablename = 'rate_limit_events' AND indexname = $1
			)
		`
		err = runner.db.QueryRow(query, idx).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check for index %s: %v", idx, err)
		}
		if !exists {
			t.Errorf("Expected index %s to exist on rate_limit_events table", idx)
		}
	}
}
