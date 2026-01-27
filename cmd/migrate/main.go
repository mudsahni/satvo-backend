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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	m, err := migrate.New("file://db/migrations", cfg.DB.DSN())
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}
	defer m.Close()

	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [up|down|steps N|version]")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migration up failed: %v", err)
		}
		log.Println("migrations applied successfully")

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migration down failed: %v", err)
		}
		log.Println("migrations reverted successfully")

	case "steps":
		if len(os.Args) < 3 {
			log.Fatal("steps requires a number argument")
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("invalid steps argument: %v", err)
		}
		if err := m.Steps(n); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migration steps failed: %v", err)
		}
		log.Printf("applied %d migration steps", n)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("failed to get version: %v", err)
		}
		fmt.Printf("version: %d, dirty: %v\n", version, dirty)

	default:
		fmt.Printf("unknown command: %s\n", cmd)
		fmt.Println("Usage: migrate [up|down|steps N|version]")
		os.Exit(1)
	}
}
