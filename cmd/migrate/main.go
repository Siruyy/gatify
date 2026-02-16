package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/Siruyy/gatify/internal/storage/migrations"
)

func main() {
	var (
		action      string
		databaseURL string
		steps       int
		version     uint
	)

	flag.StringVar(&action, "action", "up", "Migration action: up | down | steps | goto | force | version")
	flag.StringVar(&databaseURL, "database-url", os.Getenv("DATABASE_URL"), "PostgreSQL/TimescaleDB connection URL")
	flag.IntVar(&steps, "steps", 0, "Number of steps for -action=steps (positive for up, negative for down)")
	flag.UintVar(&version, "version", 0, "Target version for -action=goto or -action=force")
	flag.Parse()

	if databaseURL == "" {
		slog.Error("DATABASE_URL is required (set env var or use -database-url)")
		os.Exit(1)
	}

	runner, err := migrations.NewRunner(databaseURL)
	if err != nil {
		slog.Error("failed to initialize migration runner", "error", err)
		os.Exit(1)
	}
	defer runner.Close()

	switch action {
	case "up":
		err = runner.Up()
	case "down":
		err = runner.Down()
	case "steps":
		if steps == 0 {
			slog.Error("-steps must be non-zero when -action=steps")
			os.Exit(1)
		}
		err = runner.Steps(steps)
	case "goto":
		err = runner.MigrateTo(version)
	case "force":
		err = runner.Force(int(version))
	case "version":
		var current uint
		var dirty bool
		current, dirty, err = runner.Version()
		if err == nil {
			fmt.Printf("version=%d dirty=%t\n", current, dirty)
		}
	default:
		slog.Error("unsupported action", "action", action)
		os.Exit(1)
	}

	if err != nil {
		slog.Error("migration action failed", "action", action, "error", err)
		os.Exit(1)
	}

	if action != "version" {
		current, dirty, versionErr := runner.Version()
		if versionErr != nil {
			slog.Error("version lookup failed after migration", "action", action, "error", versionErr)
			os.Exit(1)
		}

		fmt.Printf("migration action %q completed (version=%d dirty=%t)\n", action, current, dirty)
	}
}
