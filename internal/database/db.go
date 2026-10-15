package database

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"dormitory-management-system/config"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Driver string
}

var (
	instance *DB
	once     sync.Once
)

// InitDB initializes the database connection and runs migrations and seeds
func InitDB(cfg *config.Config) (*DB, error) {
	var err error
	once.Do(func() {
		var db *sql.DB
		driver := cfg.DBDriver

		if driver == "postgres" {
			var connStr string
			if cfg.DBURL != "" {
				connStr = cfg.DBURL
			} else {
				connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
					cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
			}
			log.Printf("[DATABASE] Connecting to PostgreSQL...")
			db, err = sql.Open("postgres", connStr)
			if err != nil {
				log.Printf("[DATABASE ERROR] Failed to open PostgreSQL: %v", err)
				return
			}
			// Test ping with timeout
			if pingErr := db.Ping(); pingErr != nil {
				log.Printf("[DATABASE WARNING] PostgreSQL ping failed: %v", pingErr)
				log.Printf("[DATABASE] Falling back to SQLite local database (dormitory.db)...")
				driver = "sqlite"
				db, err = sql.Open("sqlite", "dormitory.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
				if err != nil {
					return
				}
			} else {
				log.Printf("[DATABASE] Successfully connected to PostgreSQL!")
			}
		} else {
			log.Printf("[DATABASE] Using SQLite driver (dormitory.db)...")
			db, err = sql.Open("sqlite", "dormitory.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
			if err != nil {
				log.Printf("[DATABASE ERROR] Failed to open SQLite: %v", err)
				return
			}
		}

		// Configure connection pool
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(5 * time.Minute)

		instance = &DB{
			DB:     db,
			Driver: driver,
		}

		// Run Migrations
		if migErr := instance.RunMigrations(); migErr != nil {
			err = fmt.Errorf("migration failed: %w", migErr)
			return
		}

		// Run Seeds
		if seedErr := instance.SeedData(cfg); seedErr != nil {
			err = fmt.Errorf("seeding failed: %w", seedErr)
			return
		}

		// Process emergency admin password reset if requested via server environment
		if resetErr := instance.ProcessEmergencyReset(cfg); resetErr != nil {
			log.Printf("[DATABASE WARNING] Emergency reset error: %v", resetErr)
		}
	})

	if err != nil {
		return nil, err
	}
	return instance, nil
}

// GetDB returns the database singleton
func GetDB() *DB {
	return instance
}

// Rebind converts '?' placeholders to PostgreSQL '$1', '$2', etc. if driver is postgres
func (db *DB) Rebind(query string) string {
	if db.Driver != "postgres" {
		return query
	}

	var sb strings.Builder
	paramIndex := 1
	inString := false
	var stringChar rune

	runes := []rune(query)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if (ch == '\'' || ch == '"') && (i == 0 || runes[i-1] != '\\') {
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				inString = false
			}
			sb.WriteRune(ch)
			continue
		}

		if !inString && ch == '?' {
			sb.WriteString("$" + strconv.Itoa(paramIndex))
			paramIndex++
		} else {
			sb.WriteRune(ch)
		}
	}

	return sb.String()
}
