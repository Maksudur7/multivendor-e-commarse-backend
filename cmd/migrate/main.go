package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/config"
)

func main() {
	fmt.Println("🚀 Starting Database Migration Runner for NeonDB...")

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	dbURL := cfg.DB.URL
	if dbURL == "" {
		dbURL = cfg.DB.DSN()
	}

	fmt.Printf("Connecting to database: %s...\n", dbURL)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("❌ Connection error: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("❌ Database ping failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Connected to NeonDB successfully!")

	migrationsDir := "db/migrations"
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		fmt.Printf("❌ Failed to read migrations directory: %v\n", err)
		os.Exit(1)
	}

	var upFiles []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".sql" && filepath.Base(f.Name())[len(f.Name())-7:] == ".up.sql" {
			upFiles = append(upFiles, filepath.Join(migrationsDir, f.Name()))
		}
	}
	sort.Strings(upFiles)

	for _, file := range upFiles {
		fmt.Printf("📦 Applying migration: %s ... ", filepath.Base(file))
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("❌ Read failed: %v\n", err)
			continue
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			fmt.Printf("❌ Execution error: %v\n", err)
		} else {
			fmt.Println("✅ DONE")
		}
	}

	fmt.Println("🎉 All migrations applied successfully to NeonDB!")
}
