package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

// Runner executes schema migrations against a PostgreSQL/TimescaleDB database.
type Runner struct {
	db      *sql.DB
	migrate *migrate.Migrate
}

// NewRunner creates a migration runner for the provided PostgreSQL connection URL.
func NewRunner(databaseURL string) (*Runner, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("migrations: database URL is required")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("migrations: failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: failed to connect to database: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: failed to initialize postgres driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationFiles, "sql")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: failed to initialize migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations: failed to create migrator: %w", err)
	}

	return &Runner{db: db, migrate: m}, nil
}

// Up runs all pending up migrations.
func (r *Runner) Up() error {
	if err := r.migrate.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: up failed: %w", err)
	}

	return nil
}

// Down rolls back all migrations.
func (r *Runner) Down() error {
	if err := r.migrate.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: down failed: %w", err)
	}

	return nil
}

// Steps applies a signed number of migration steps.
func (r *Runner) Steps(steps int) error {
	if steps == 0 {
		return nil
	}

	if err := r.migrate.Steps(steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: steps failed: %w", err)
	}

	return nil
}

// MigrateTo migrates to an exact version.
func (r *Runner) MigrateTo(version uint) error {
	if err := r.migrate.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: migrate to version %d failed: %w", version, err)
	}

	return nil
}

// Force sets the migration version without running migrations.
func (r *Runner) Force(version int) error {
	if err := r.migrate.Force(version); err != nil {
		return fmt.Errorf("migrations: force version %d failed: %w", version, err)
	}

	return nil
}

// Version returns current schema version and dirty flag.
func (r *Runner) Version() (uint, bool, error) {
	version, dirty, err := r.migrate.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}

		return 0, false, fmt.Errorf("migrations: failed to fetch version: %w", err)
	}

	return version, dirty, nil
}

// Close releases migration and database resources.
func (r *Runner) Close() {
	if r.migrate != nil {
		sourceErr, dbErr := r.migrate.Close()
		if sourceErr != nil {
			slog.Warn("error closing migration source", "error", sourceErr)
		}
		if dbErr != nil {
			slog.Warn("error closing migration database", "error", dbErr)
		}
	}

	if r.db != nil {
		if err := r.db.Close(); err != nil {
			slog.Warn("error closing database connection", "error", err)
		}
	}
}
