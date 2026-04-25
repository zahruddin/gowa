package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dataSourceName string) *sql.DB {
	var err error
	// Tambahkan foreign_keys=on agar hubungan antar tabel (FK) aktif
	DB, err = sql.Open("sqlite3", dataSourceName+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	DB.SetMaxOpenConns(1)
	DB.SetMaxIdleConns(1)

	createTables()
	return DB
}

func createTables() {
	// Urutan pembuatan sangat penting: Parent table (sessions) harus dibuat duluan
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY, 
			api_token TEXT UNIQUE,
			webhook_url TEXT,
			device_jid TEXT,
			status TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Migration: add device_jid column if table already exists without it
		`ALTER TABLE sessions ADD COLUMN device_jid TEXT;`,

		`CREATE TABLE IF NOT EXISTS katalog (
			session_id TEXT,
			keyword TEXT,
			details TEXT,
			PRIMARY KEY (session_id, keyword),
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS whitelist (
			session_id TEXT,
			number TEXT,
			name TEXT,
			PRIMARY KEY (session_id, number),
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS bulk_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			total INTEGER,
			success INTEGER,
			failed INTEGER,
			status TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE,
			password TEXT
		);`,

		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			// Ignore ALTER TABLE errors (column may already exist)
			if strings.Contains(query, "ALTER TABLE") {
				continue
			}
			log.Fatalf("Failed to create table: %v\nQuery: %s", err, query)
		}
	}
	fmt.Println("Database tables initialized with multi-session support.")
}