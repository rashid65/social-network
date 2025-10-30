package sqlite_test

import (
	"fmt"
	"path/filepath"
	"social-network/pkg/db/sqlite"
	"testing"
)

func TestRunMigrations(t *testing.T) {
	// Create a temporary SQLite database file
	dbPath := filepath.Join(t.TempDir(), "test.db")
	fmt.Printf("Test database created at: %s\n", dbPath)

	// path to the migrations directory
	migrationDir := "../migrations/sqlite"

	// Run migrations
	err := sqlite.RunMigrations(dbPath, migrationDir)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
}
