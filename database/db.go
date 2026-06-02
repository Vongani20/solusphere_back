package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// InitDB initializes database using environment variables (backward compatibility)
func InitDB() {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")

	InitDBWithSecrets(dbUser, dbPassword, dbHost, dbName)
}

// InitDBWithSecrets initializes database with provided credentials
func InitDBWithSecrets(dbUser, dbPassword, dbHost, dbName string) {
	log.Println(" InitDBWithSecrets called")
	log.Printf("   Parameters: dbUser=%s, dbHost=%s, dbName=%s", dbUser, dbHost, dbName)

	var err error

	// MySQL connection string format
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		dbUser, dbPassword, dbHost, dbName)

	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(" Failed to connect to database:", err)
	}

	// Set connection pool settings
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * 60 * time.Second) // 5 minutes

	err = DB.Ping()
	if err != nil {
		log.Fatal(" Failed to ping database:", err)
	}

	log.Println(" Connected to MySQL database successfully!")
	log.Printf(" Database: %s@%s/%s", dbUser, dbHost, dbName)

	// Log the memory address of DB for debugging
	log.Printf(" DB memory address: %p", DB)
}

// GetDBConnection returns the current DB connection for debugging
func GetDBConnection() *sql.DB {
	return DB
}

// GetDBStatus returns detailed status of the database connection
func GetDBStatus() string {
	if DB == nil {
		return "nil (not initialized)"
	}

	if err := DB.Ping(); err != nil {
		if err.Error() == "sql: database is closed" {
			return "CLOSED"
		}
		return "error: " + err.Error()
	}

	return "healthy"
}

// EnsureDB checks and reconnects if necessary
func EnsureDB() error {
	if DB == nil {
		log.Println(" DB is nil, reinitializing...")
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbHost := os.Getenv("DB_HOST")
		dbName := os.Getenv("DB_NAME")
		InitDBWithSecrets(dbUser, dbPassword, dbHost, dbName)
		return nil
	}

	if err := DB.Ping(); err != nil {
		log.Printf(" DB Ping failed: %v, reinitializing...", err)
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbHost := os.Getenv("DB_HOST")
		dbName := os.Getenv("DB_NAME")
		InitDBWithSecrets(dbUser, dbPassword, dbHost, dbName)
		return err
	}

	return nil
}

// GetDBConnectionString returns the MySQL connection string (useful for debugging)
func GetDBConnectionString(dbUser, dbPassword, dbHost, dbName string) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true",
		dbUser, "********", dbHost, dbName) // Password masked for security
}

// CloseDatabase closes the database connection (use only for shutdown)
func CloseDatabase() {
	if DB != nil {
		log.Println("Closing database connection...")
		DB.Close()
	}
}

// ReconnectDatabase re-establishes the database connection
func ReconnectDatabase(dbUser, dbPassword, dbHost, dbName string) error {
	log.Println("Reconnecting to database...")
	InitDBWithSecrets(dbUser, dbPassword, dbHost, dbName)
	return DB.Ping()
}
