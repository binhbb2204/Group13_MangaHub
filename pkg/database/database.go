package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDatabase(dbPath string) error {
	// Ensure parent directory exists so sqlite can create the .db file
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}
	}

	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	log.Println("Database connection established")

	// Enable foreign key support for SQLite (useful for cascading deletes, etc.)
	if _, err := DB.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Printf("Warning: failed to enable foreign keys: %v", err)
	}

	if err = createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Database tables created successfully")
	return nil
}

func createTables() error {
	schema := `
    CREATE TABLE IF NOT EXISTS users (
        id TEXT PRIMARY KEY,
        username TEXT UNIQUE NOT NULL,
        email TEXT UNIQUE,
        password_hash TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS manga (
        id TEXT PRIMARY KEY,
        title TEXT NOT NULL,
        author TEXT,
        genres TEXT,
        status TEXT,
        total_chapters INTEGER DEFAULT 0,
        description TEXT,
        cover_url TEXT
    );

    CREATE TABLE IF NOT EXISTS user_progress (
        user_id TEXT NOT NULL,
        manga_id TEXT NOT NULL,
        current_chapter INTEGER DEFAULT 0,
        status TEXT DEFAULT 'plan_to_read',
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, manga_id),
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
        FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
    );

    CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
    CREATE INDEX IF NOT EXISTS idx_manga_author ON manga(author);
    CREATE INDEX IF NOT EXISTS idx_user_progress_user ON user_progress(user_id);
    `

	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}
	// Backfill: ensure email column exists for existing DBs
	if err := ensureUserEmailColumn(); err != nil {
		return err
	}
	return nil
}

func ensureUserEmailColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(users);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasEmail := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, "email") {
			hasEmail = true
			break
		}
	}
	if !hasEmail {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN email TEXT UNIQUE;`); err != nil {
			log.Printf("Warning: adding email column failed: %v", err)
		}
	}
	return nil
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
