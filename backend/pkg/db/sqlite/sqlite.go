package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

// WAL configuration constants
const (
	// WAL checkpoint interval (number of pages)
	WAL_AUTOCHECKPOINT = 1000
	// Synchronous mode for WAL
	SYNCHRONOUS_NORMAL = "NORMAL"
	// Busy timeout in milliseconds
	BUSY_TIMEOUT = 5000
)

// OpenConnection opens a SQLite database connection with WAL mode enabled
func OpenConnection(dbPath string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Connection string with WAL-optimized parameters
	connStr := fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=on&_busy_timeout=%d",
		dbPath, BUSY_TIMEOUT)

	// Open database connection
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings for concurrent access
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection and configure WAL mode
	if err := configureWALMode(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure WAL mode: %w", err)
	}

	return db, nil
}

// configureWALMode ensures WAL mode is properly configured
func configureWALMode(db *sql.DB) error {
	// Set PRAGMAs that don't return scannable values
	execPragmas := []struct {
		name  string
		value string
	}{
		{"synchronous", SYNCHRONOUS_NORMAL},
		{"cache_size", "1000"},
		{"foreign_keys", "ON"},
		{"temp_store", "memory"},
		{"mmap_size", "268435456"}, // 256MB
		{"wal_autocheckpoint", fmt.Sprintf("%d", WAL_AUTOCHECKPOINT)},
	}

	// Execute PRAGMAs that don't return values
	for _, pragma := range execPragmas {
		query := fmt.Sprintf("PRAGMA %s = %s", pragma.name, pragma.value)
		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to set PRAGMA %s: %w", pragma.name, err)
		}
	}

	// Set and verify journal_mode (this one returns a value)
	var journalMode string
	err := db.QueryRow("PRAGMA journal_mode = WAL").Scan(&journalMode)
	if err != nil {
		return fmt.Errorf("failed to set PRAGMA journal_mode: %w", err)
	}

	// Verify WAL mode was enabled
	if journalMode != "wal" {
		return fmt.Errorf("failed to enable WAL mode, got: %s", journalMode)
	}

	log.Printf("PRAGMA journal_mode = WAL (result: %s)", journalMode)
	return nil
}

// CheckWALMode verifies that WAL mode is enabled
func CheckWALMode(db *sql.DB) (bool, error) {
	var journalMode string
	err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		return false, fmt.Errorf("failed to check journal mode: %w", err)
	}

	return journalMode == "wal", nil
}

// WALCheckpoint manually triggers a WAL checkpoint
func WALCheckpoint(db *sql.DB) error {
	_, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %w", err)
	}

	log.Println("WAL checkpoint completed")
	return nil
}

// GetWALInfo returns information about the current WAL state
func GetWALInfo(db *sql.DB) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get WAL size
	var walSize int64
	err := db.QueryRow("PRAGMA wal_checkpoint").Scan(&walSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get WAL info: %w", err)
	}

	// Get journal mode
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		return nil, fmt.Errorf("failed to get journal mode: %w", err)
	}

	// Get synchronous mode
	var syncMode string
	err = db.QueryRow("PRAGMA synchronous").Scan(&syncMode)
	if err != nil {
		return nil, fmt.Errorf("failed to get synchronous mode: %w", err)
	}

	info["wal_size"] = walSize
	info["journal_mode"] = journalMode
	info["synchronous"] = syncMode

	return info, nil
}

// RunMigrations runs database migrations with WAL mode support
func RunMigrations(dbPath string, migrationsDir string) error {
	// Open connection with WAL mode
	db, err := OpenConnection(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	// Verify WAL mode is enabled
	isWAL, err := CheckWALMode(db)
	if err != nil {
		return fmt.Errorf("failed to verify WAL mode: %w", err)
	}
	if !isWAL {
		return fmt.Errorf("WAL mode is not enabled")
	}

	log.Println("Database opened with WAL mode enabled")

	// Create driver instance for migrations
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create driver instance: %w", err)
	}

	// Get absolute path for migration directory
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create migration instance
	sourceURL := fmt.Sprintf("file://%s", filepath.ToSlash(absPath))
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Checkpoint WAL after migrations
	if err := WALCheckpoint(db); err != nil {
		log.Printf("Warning: failed to checkpoint WAL after migrations: %v", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// ForceVersion sets the migration version without running migrations
func ForceVersion(dbPath string, migrationsDir string, version uint) error {
	m, err := createMigrationInstance(dbPath, migrationsDir)
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Force(int(version))
	if err != nil {
		return fmt.Errorf("Failed to force version %d: %w", version, err)
	}

	fmt.Printf("Forced migaration version to: %d\n", version)
	return nil
}
