package database

import (
	"log"
)

// RunMigrations ensures all tables and indexes exist in the connected database
func (db *DB) RunMigrations() error {
	log.Printf("[DATABASE] Running migrations for driver: %s", db.Driver)

	var schemaSQL string

	if db.Driver == "postgres" {
		schemaSQL = `
		CREATE TABLE IF NOT EXISTS rooms (
			id SERIAL PRIMARY KEY,
			room_number VARCHAR(50) NOT NULL UNIQUE,
			room_type VARCHAR(50) NOT NULL,
			price NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
			floor INTEGER NOT NULL DEFAULT 1,
			status VARCHAR(30) NOT NULL DEFAULT 'vacant',
			description TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(150) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			role VARCHAR(30) NOT NULL DEFAULT 'tenant',
			phone VARCHAR(50),
			room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS maintenance_requests (
			id SERIAL PRIMARY KEY,
			tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
			title VARCHAR(200) NOT NULL,
			description TEXT NOT NULL,
			category VARCHAR(50) NOT NULL DEFAULT 'other',
			status VARCHAR(30) NOT NULL DEFAULT 'pending',
			admin_notes TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS payments (
			id SERIAL PRIMARY KEY,
			tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
			billing_month VARCHAR(20) NOT NULL,
			amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
			payment_date VARCHAR(30) NOT NULL,
			slip_url VARCHAR(255),
			status VARCHAR(30) NOT NULL DEFAULT 'pending',
			rejection_reason TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
		CREATE INDEX IF NOT EXISTS idx_rooms_status ON rooms(status);
		CREATE INDEX IF NOT EXISTS idx_maintenance_tenant ON maintenance_requests(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_payments_tenant ON payments(tenant_id);
		`
	} else {
		// SQLite compatible schema
		schemaSQL = `
		CREATE TABLE IF NOT EXISTS rooms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			room_number TEXT NOT NULL UNIQUE,
			room_type TEXT NOT NULL,
			price REAL NOT NULL DEFAULT 0.00,
			floor INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'vacant',
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'tenant',
			phone TEXT,
			room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS maintenance_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'other',
			status TEXT NOT NULL DEFAULT 'pending',
			admin_notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
			billing_month TEXT NOT NULL,
			amount REAL NOT NULL DEFAULT 0.00,
			payment_date TEXT NOT NULL,
			slip_url TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			rejection_reason TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
		CREATE INDEX IF NOT EXISTS idx_rooms_status ON rooms(status);
		CREATE INDEX IF NOT EXISTS idx_maintenance_tenant ON maintenance_requests(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_payments_tenant ON payments(tenant_id);
		`
	}

	_, err := db.Exec(schemaSQL)
	if err != nil {
		log.Printf("[DATABASE ERROR] Migration execution failed: %v", err)
		return err
	}

	log.Printf("[DATABASE] Migrations executed successfully")
	return nil
}
