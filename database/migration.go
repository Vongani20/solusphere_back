package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations executes database migrations using SQL files
func RunMigrations(db *sql.DB) error {
	// Ensure migrations directory exists
	if err := ensureMigrationsDirectory(); err != nil {
		log.Printf(" Could not create migrations directory: %v", err)
		// Create an empty migration file to allow the app to start
		if err := createEmptyMigration(); err != nil {
			log.Printf(" Could not create empty migration: %v", err)
		}
	}

	// Get the migration path - try multiple locations
	migrationPath := findMigrationPath()

	// Use a relative path with forward slashes
	relativePath := "./migrations"

	// On Windows, convert to proper format
	sourceURL := "file://" + filepath.ToSlash(relativePath)
	log.Printf("Using migration source: %s", sourceURL)
	log.Printf("Absolute path: %s", migrationPath)

	// Check if migrations directory is empty
	if isEmpty, _ := isMigrationsDirectoryEmpty(); isEmpty {
		log.Println(" No migration files found, creating initial migration")
		if err := CreateMigration("initial_schema"); err != nil {
			log.Printf(" Could not create initial migration: %v", err)
			log.Println(" Skipping migrations - app will start without schema changes")
			return nil
		}
	}

	// Create driver instance
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	// Create migration instance with relative path
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"mysql", driver)
	if err != nil {
		return fmt.Errorf("could not create migration instance: %v", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not run migrations: %v", err)
	}

	log.Println(" Database migrations completed successfully")
	return nil
}

// ensureMigrationsDirectory creates the migrations directory if it doesn't exist
func ensureMigrationsDirectory() error {
	if _, err := os.Stat("./migrations"); os.IsNotExist(err) {
		log.Println(" Creating migrations directory...")
		return os.MkdirAll("./migrations", 0755)
	}
	return nil
}

// isMigrationsDirectoryEmpty checks if the migrations directory has any .sql files
func isMigrationsDirectoryEmpty() (bool, error) {
	files, err := os.ReadDir("./migrations")
	if err != nil {
		return true, err
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sql" {
			return false, nil
		}
	}
	return true, nil
}

// createEmptyMigration creates a placeholder migration if none exist
func createEmptyMigration() error {
	if err := ensureMigrationsDirectory(); err != nil {
		return err
	}

	// Check if any migration files already exist
	if exists, _ := isMigrationsDirectoryEmpty(); !exists {
		return nil
	}

	// Create a placeholder migration
	return CreateMigration("placeholder")
}

// findMigrationPath locates the migrations directory
func findMigrationPath() string {
	// Try current directory first
	if _, err := os.Stat("./migrations"); err == nil {
		absPath, _ := filepath.Abs("./migrations")
		return absPath
	}

	// Try parent directory
	if _, err := os.Stat("../migrations"); err == nil {
		absPath, _ := filepath.Abs("../migrations")
		return absPath
	}

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "./migrations"
	}

	// Try full path
	fullPath := filepath.Join(cwd, "migrations")
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	return "./migrations"
}

// RunMigrationsToVersion runs migrations up to a specific version
func RunMigrationsToVersion(db *sql.DB, version uint) error {
	// Ensure migrations directory exists
	ensureMigrationsDirectory()

	sourceURL := "file://./migrations"

	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"mysql", driver)
	if err != nil {
		return fmt.Errorf("could not create migration instance: %v", err)
	}
	defer m.Close()

	if err := m.Migrate(version); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not run migrations to version %d: %v", version, err)
	}

	log.Printf(" Database migrations to version %d completed successfully", version)
	return nil
}

// RollbackMigrations rolls back the last migration
func RollbackMigrations(db *sql.DB) error {
	// Ensure migrations directory exists
	ensureMigrationsDirectory()

	sourceURL := "file://./migrations"

	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"mysql", driver)
	if err != nil {
		return fmt.Errorf("could not create migration instance: %v", err)
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not rollback migrations: %v", err)
	}

	log.Println(" Database rollback completed successfully")
	return nil
}

// ForceMigrationVersion forces the migration version
func ForceMigrationVersion(db *sql.DB, version int) error {
	// Ensure migrations directory exists
	ensureMigrationsDirectory()

	sourceURL := "file://./migrations"

	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("could not create database driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"mysql", driver)
	if err != nil {
		return fmt.Errorf("could not create migration instance: %v", err)
	}
	defer m.Close()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("could not force migration version: %v", err)
	}

	log.Printf(" Migration version forced to %d", version)
	return nil
}

// CreateMigration creates new migration files
func CreateMigration(name string) error {
	// Ensure migrations directory exists
	if err := os.MkdirAll("./migrations", 0755); err != nil {
		return fmt.Errorf("could not create migrations directory: %v", err)
	}

	// Get next version number
	version := getNextVersion("./migrations")

	// Create up file
	upFilePath := filepath.Join("./migrations", fmt.Sprintf("%06d_%s.up.sql", version, name))
	upFile, err := os.Create(upFilePath)
	if err != nil {
		return fmt.Errorf("could not create up file: %v", err)
	}
	defer upFile.Close()

	// Create down file
	downFilePath := filepath.Join("./migrations", fmt.Sprintf("%06d_%s.down.sql", version, name))
	downFile, err := os.Create(downFilePath)
	if err != nil {
		return fmt.Errorf("could not create down file: %v", err)
	}
	defer downFile.Close()

	// Add boilerplate comments with some basic schema
	upFile.WriteString(fmt.Sprintf("-- Migration %d %s (UP)\n", version, name))
	upFile.WriteString("\n-- Write your UP migration SQL here\n\n")

	downFile.WriteString(fmt.Sprintf("-- Migration %d %s (DOWN)\n", version, name))
	downFile.WriteString("\n-- Write your DOWN migration SQL here\n\n")

	log.Printf("✅ Created migration files for version %d: %s", version, name)
	return nil
}

// getNextVersion gets the next migration version number
func getNextVersion(migrationPath string) int {
	maxVersion := 0

	// Read all files in migrations directory
	files, err := os.ReadDir(migrationPath)
	if err != nil {
		return 1
	}

	for _, file := range files {
		if !file.IsDir() {
			var version int
			_, err := fmt.Sscanf(file.Name(), "%d_", &version)
			if err == nil && version > maxVersion {
				maxVersion = version
			}
		}
	}

	return maxVersion + 1
}
