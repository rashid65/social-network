package sqlite

import (
    "database/sql"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

// BackupDatabase creates a backup of the database in WAL mode
func BackupDatabase(db *sql.DB, sourcePath, backupPath string) error {
    // Ensure backup directory exists
    backupDir := filepath.Dir(backupPath)
    if err := os.MkdirAll(backupDir, 0755); err != nil {
        return fmt.Errorf("failed to create backup directory: %w", err)
    }

    // Checkpoint WAL to ensure all data is in the main database file
    if err := WALCheckpoint(db); err != nil {
        return fmt.Errorf("failed to checkpoint WAL before backup: %w", err)
    }

    // Create backup using SQLite's backup API
    backupDB, err := sql.Open("sqlite3", backupPath)
    if err != nil {
        return fmt.Errorf("failed to open backup database: %w", err)
    }
    defer backupDB.Close()

    // Use VACUUM INTO for atomic backup
    query := fmt.Sprintf("VACUUM INTO '%s'", backupPath)
    _, err = db.Exec(query)
    if err != nil {
        return fmt.Errorf("failed to create backup: %w", err)
    }

    return nil
}

// CopyDatabaseFiles copies database files including WAL and SHM files
func CopyDatabaseFiles(sourcePath, destPath string) error {
    files := []string{
        sourcePath,           // main database file
        sourcePath + "-wal",  // WAL file
        sourcePath + "-shm",  // SHM file
    }

    for _, file := range files {
        if _, err := os.Stat(file); os.IsNotExist(err) {
            continue // Skip if file doesn't exist
        }

        destFile := filepath.Join(filepath.Dir(destPath), filepath.Base(file))
        if err := copyFile(file, destFile); err != nil {
            return fmt.Errorf("failed to copy %s: %w", file, err)
        }
    }

    return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
    source, err := os.Open(src)
    if err != nil {
        return err
    }
    defer source.Close()

    destination, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer destination.Close()

    _, err = io.Copy(destination, source)
    return err
}