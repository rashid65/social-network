package main

import (
    "flag"
    "fmt"
    "log"
    "os"

    "social-network/pkg/db/sqlite"
)

func main() {
    var (
        dbPath        = flag.String("db", "./social-network.db", "Database file path")
        migrationsDir = flag.String("migrations", "./pkg/db/migrations/sqlite", "Migrations directory")
        action        = flag.String("action", "up", "Migration action: up, down, status, rollback, force")
        steps         = flag.Int("steps", 0, "Number of migration steps (0 = all)")
        version       = flag.Uint("version", 0, "Target version for rollback/force")
    )
    flag.Parse()

    switch *action {
    case "up":
        if err := sqlite.RunMigrations(*dbPath, *migrationsDir); err != nil {
            log.Fatal("Failed to run migrations:", err)
        }
        fmt.Println("Migrations applied successfully")

    case "down":
        if *steps > 0 {
            if err := sqlite.RollbackMigrations(*dbPath, *migrationsDir, *steps); err != nil {
                log.Fatal("Failed to rollback migrations:", err)
            }
        } else {
            if err := sqlite.RollbackAll(*dbPath, *migrationsDir); err != nil {
                log.Fatal("Failed to rollback all migrations:", err)
            }
        }

    case "rollback":
        if *version > 0 {
            if err := sqlite.RollbackToVersion(*dbPath, *migrationsDir, *version); err != nil {
                log.Fatal("Failed to rollback to version:", err)
            }
        } else if *steps > 0 {
            if err := sqlite.RollbackMigrations(*dbPath, *migrationsDir, *steps); err != nil {
                log.Fatal("Failed to rollback migrations:", err)
            }
        } else {
            log.Fatal("Please specify either -version or -steps for rollback")
        }

    case "force":
        if *version == 0 {
            log.Fatal("Please specify -version for force action")
        }
        if err := sqlite.ForceVersion(*dbPath, *migrationsDir, *version); err != nil {
            log.Fatal("Failed to force version:", err)
        }

    case "status":
        version, dirty, err := sqlite.GetMigrationVersion(*dbPath, *migrationsDir)
        if err != nil {
            log.Fatal("Failed to get migration status:", err)
        }
        fmt.Printf("Current migration version: %d\n", version)
        if dirty {
            fmt.Println("Database state: DIRTY (needs manual intervention)")
        } else {
            fmt.Println("Database state: CLEAN")
        }

    default:
        fmt.Printf("Invalid action: %s\n", *action)
        fmt.Println("Valid actions: up, down, rollback, force, status")
        os.Exit(1)
    }
}