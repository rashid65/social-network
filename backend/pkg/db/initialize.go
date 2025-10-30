package db

import (
	"database/sql"
	"log"
	"social-network/pkg/db/sqlite"
)

var DB *sql.DB

// Initialize sets up the database connections and run migrations
func Initialize(dbPath string, migrationsDir string) error {
	var err error

	// open connection with WAL mode
	DB, err = sqlite.OpenConnection(dbPath)
	if err != nil {
		return err
	}

	// Run migration first
	err = sqlite.RunMigrations(dbPath, migrationsDir)
	if err != nil {
		DB.Close()
		return err
	}

	// Log WAL status
	isWAL, err := sqlite.CheckWALMode(DB)
	if err != nil {
		log.Printf("Failed to check WAL mode: %v", err)
	} else {
		log.Printf("WAL mode enabled: %v", isWAL)
	}

	// Open the database connection
	// db, err := sql.Open("sqlite3", dbPath)
	// if err != nil {
	// 	return err
	// }

	// Set connection pool settings
	// db.SetMaxOpenConns(25)
	// db.SetMaxIdleConns(25)

	// test the connection
	// if err := db.Ping(); err != nil {
	// 	return err
	// }

	// DB = db
	// log.Println("Database connection established")
	return nil
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		// Checkpoint WAL before closing
		if err := sqlite.WALCheckpoint(DB); err != nil {
			log.Printf("Warning: failed to checkpoint WAL before closing: %v", err)
		}
		// Close the database connection
		return DB.Close()
	}
	return nil
}