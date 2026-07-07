// Package database provides PostgreSQL connection management and migration support.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	// PostgreSQL driver
	_ "github.com/lib/pq"

	"github.com/codepilot-ai/codepilot-ai/internal/config"
	"github.com/codepilot-ai/codepilot-ai/pkg/logger"
)

// NewPostgresDB creates a new PostgreSQL connection pool with the given configuration.
// It pings the database to verify connectivity before returning.
func NewPostgresDB(cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Verify connectivity with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("database", cfg.DBName).
		Int("max_open_conns", cfg.MaxOpenConns).
		Int("max_idle_conns", cfg.MaxIdleConns).
		Msg("connected to PostgreSQL")

	return db, nil
}

// RunMigrations reads SQL migration files from the specified directory and executes
// them against the database in sorted order. Migration files must have a .sql extension.
func RunMigrations(db *sql.DB) error {
	migrationsDir := findMigrationsDir()

	logger.Log.Info().Str("directory", migrationsDir).Msg("running database migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory '%s': %w", migrationsDir, err)
	}

	// Collect and sort .sql files
	var migrationFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".sql" {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	if len(migrationFiles) == 0 {
		logger.Log.Warn().Str("directory", migrationsDir).Msg("no migration files found")
		return nil
	}

	for _, filename := range migrationFiles {
		filePath := filepath.Join(migrationsDir, filename)

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file '%s': %w", filename, err)
		}

		if len(content) == 0 {
			logger.Log.Warn().Str("file", filename).Msg("skipping empty migration file")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, execErr := db.ExecContext(ctx, string(content))
		cancel()

		if execErr != nil {
			return fmt.Errorf("failed to execute migration '%s': %w", filename, execErr)
		}

		logger.Log.Info().Str("file", filename).Msg("migration applied successfully")
	}

	logger.Log.Info().Int("count", len(migrationFiles)).Msg("all migrations applied")
	return nil
}

// findMigrationsDir determines the migrations directory path.
// Checks root migrations/ first (Docker working dir), then internal/ for local dev.
func findMigrationsDir() string {
	candidates := []string{
		"migrations",
		"./migrations",
		"internal/database/migrations",
		"./internal/database/migrations",
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return "migrations"
}
