package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/glebarez/go-sqlite"
)

var DB *sql.DB

func InitDatabase(dbPath string) error {
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	log.Println("Database connection established")

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
	return err
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
