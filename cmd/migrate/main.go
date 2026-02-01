package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"satvos/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [up|down|steps N|version]")
		os.Exit(1)
	}

	m, err := migrate.New("file://db/migrations", cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration up failed: %w", err)
		}
		log.Println("migrations applied successfully")

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration down failed: %w", err)
		}
		log.Println("migrations reverted successfully")

	case "steps":
		if len(os.Args) < 3 {
			return fmt.Errorf("steps requires a number argument")
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			return fmt.Errorf("invalid steps argument: %w", err)
		}
		if err := m.Steps(n); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration steps failed: %w", err)
		}
		log.Printf("applied %d migration steps", n)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			return fmt.Errorf("failed to get version: %w", err)
		}
		fmt.Printf("version: %d, dirty: %v\n", version, dirty)

	default:
		return fmt.Errorf("unknown command: %s\nUsage: migrate [up|down|steps N|version]", cmd)
	}

	return nil
}
