package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"      // Fix: Add this import
    _ "github.com/mattn/go-sqlite3"
)

// rollback to specific number of steps
func RollbackMigrations(dbPath string, migrationDir string, steps int) error {
	m, err := createMigrationInstance(dbPath, migrationDir)
	if err != nil {
		return err
	}
	defer m.Close()

	// Get current version
	currentVersion, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("Failed to get current migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("Database is in a dirty state, please resolve the issue before rolling back")
	}

	fmt.Printf("Current migration version: %d\n", currentVersion)

	// calculate target version
	targetVersion := int(currentVersion) - steps
	if targetVersion < 0 {
		return fmt.Errorf("Cannot roll back to a version less than 0")
	}

	if targetVersion == 0 {
		// rollback to initial state
		err = m.Down()
	} else {
		// rollback to specific version
		err = m.Migrate(uint(targetVersion))
	}

	if err != nil {
		return fmt.Errorf("Failed to roll back migrations: %w", err)
	}

	fmt.Printf("Successfully rolled back to version: %d\n", targetVersion)
	return nil
}

// rolls back to specific version
func RollbackToVersion(dbPath string, migrationsDir string, version uint) error {
	m, err := createMigrationInstance(dbPath, migrationsDir)
	if err != nil {
		return err
	}
	defer m.Close()

	currentVersion, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("Failed to get current migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("Database is in a dirty state, please resolve the issue before rolling back")
	}

	if version > currentVersion {
		return fmt.Errorf("Cannot roll back to a version greater than the current version (%d)", currentVersion)
	}

	err = m.Migrate(version)
	if err != nil {
		return fmt.Errorf("Failed to roll back to version %d: %w", version, err)
	}

	fmt.Printf("Successfully rolled back to version: %d\n", version)
	return nil
}

// rollback to initial state
func RollbackAll(dbPath string, migrationsDir string) error {
	m, err := createMigrationInstance(dbPath, migrationsDir)
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Down()
	if err != nil {
		return fmt.Errorf("Failed to roll back all migrations: %w", err)
	}

	fmt.Println("Successfully rolled back all migrations to the initial state")
	return nil
}

func GetMigrationVersion(dbPath string, migrationsDir string) (uint, bool, error) {
	m, err := createMigrationInstance(dbPath, migrationsDir)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return 0, false, fmt.Errorf("Failed to get current migration version: %w", err)
	}

	return version, dirty, nil
}

// helper function to create a migration instance
func createMigrationInstance(dbPath string, migrationsDir string) (*migrate.Migrate, error) {
	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database: %w", err)
	}

	// Create database driver instance
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Failed to create driver instance: %w", err)
	}

	// Get absolute path for migration directory
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Failed to get absolute path: %w", err)
	}

	// Create migration instance
	sourceURL := fmt.Sprintf("file://%s", filepath.ToSlash(absPath))
	fmt.Printf("Creating migration instance with source URL: %s\n", sourceURL)
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"sqlite3",
		driver,
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Failed to create migration instance: %w", err)
	}

	return m, nil
}
