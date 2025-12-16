package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
	// _ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDatabase(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	log.Println("Database connection established")

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
        cover_url TEXT,
        media_type TEXT DEFAULT 'manga',
        mangadex_id TEXT
    );

    CREATE TABLE IF NOT EXISTS user_progress (
        user_id TEXT NOT NULL,
        manga_id TEXT NOT NULL,
        current_chapter INTEGER DEFAULT 0,
        status TEXT DEFAULT 'plan_to_read',
        user_rating REAL,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, manga_id),
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
        FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS chat_messages (
        id TEXT PRIMARY KEY,
        from_user_id TEXT NOT NULL,
        to_user_id TEXT,
        content TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (from_user_id) REFERENCES users(id) ON DELETE CASCADE,
        FOREIGN KEY (to_user_id) REFERENCES users(id) ON DELETE CASCADE
    );

    CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
    CREATE INDEX IF NOT EXISTS idx_manga_author ON manga(author);
    CREATE INDEX IF NOT EXISTS idx_user_progress_user ON user_progress(user_id);
    `

	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}
	// Migration for existing DBs that don't have media_type column
	if err := ensureMediaTypeColumn(); err != nil {
		return err
	}
	// Migration for existing DBs that don't have mangadex_id column
	if err := ensureMangaDexIDColumn(); err != nil {
		return err
	}
	return nil
}

func ensureMediaTypeColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(manga);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasMediaType := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, "media_type") {
			hasMediaType = true
			break
		}
	}
	if !hasMediaType {
		if _, err := DB.Exec(`ALTER TABLE manga ADD COLUMN media_type TEXT DEFAULT 'manga';`); err != nil {
			log.Printf("Warning: adding media_type column failed: %v", err)
		} else {
			log.Println("✓ Added media_type column to existing database")
		}
	}
	return nil
}

func ensureMangaDexIDColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(manga);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasMangaDexID := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, "mangadex_id") {
			hasMangaDexID = true
			break
		}
	}
	if !hasMangaDexID {
		if _, err := DB.Exec(`ALTER TABLE manga ADD COLUMN mangadex_id TEXT;`); err != nil {
			log.Printf("Warning: adding mangadex_id column failed: %v", err)
		} else {
			log.Println("✓ Added mangadex_id column to existing database")
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
