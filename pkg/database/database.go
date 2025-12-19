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
        role TEXT DEFAULT 'member',
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
		media_type TEXT DEFAULT 'manga'
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

    CREATE TABLE IF NOT EXISTS conversations (
        id TEXT PRIMARY KEY,
        name TEXT UNIQUE NOT NULL,
        type TEXT CHECK(type IN ('global', 'manga', 'custom')) NOT NULL,
        manga_id TEXT,
        created_by TEXT,
        last_message_at DATETIME,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
    );

    CREATE TABLE IF NOT EXISTS messages (
        id TEXT PRIMARY KEY,
        conversation_id TEXT NOT NULL,
        sender_id TEXT NOT NULL,
        content TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
        FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS user_conversation_history (
        user_id TEXT NOT NULL,
        conversation_id TEXT NOT NULL,
        last_read_message_id TEXT,
        unread_count INTEGER DEFAULT 0,
        joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, conversation_id),
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
        FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
        FOREIGN KEY (last_read_message_id) REFERENCES messages(id) ON DELETE SET NULL
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

    -- Per-conversation membership and roles
    CREATE TABLE IF NOT EXISTS user_conversation_history (
        user_id TEXT NOT NULL,
        conversation_id TEXT NOT NULL,
        last_read_message_id TEXT,
        unread_count INTEGER DEFAULT 0,
        role TEXT DEFAULT 'member',
        joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (user_id, conversation_id)
    );

	CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
	CREATE INDEX IF NOT EXISTS idx_manga_author ON manga(author);
    CREATE INDEX IF NOT EXISTS idx_user_progress_user ON user_progress(user_id);
    CREATE INDEX IF NOT EXISTS idx_conversations_type ON conversations(type);
    CREATE INDEX IF NOT EXISTS idx_conversations_manga_id ON conversations(manga_id);
    CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at DESC);
    CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);
    CREATE INDEX IF NOT EXISTS idx_user_conversation_history_user ON user_conversation_history(user_id);
    CREATE INDEX IF NOT EXISTS idx_user_conversation_history_conv ON user_conversation_history(conversation_id);
    `

	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}

	// Ensure global conversation exists
	DB.Exec(`INSERT OR IGNORE INTO conversations (id, name, type, created_at, last_message_at)
		VALUES ('global', 'global', 'global', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)

	// Migration for existing DBs that don't have media_type column
	if err := ensureMediaTypeColumn(); err != nil {
		return err
	}

	// Migration for existing DBs that don't have user_rating in user_progress
	if err := ensureUserProgressRatingColumn(); err != nil {
		return err
	}

	// Migration for existing DBs that don't have role columns
	if err := ensureUserRoleColumn(); err != nil {
		return err
	}
	if err := ensureUserConversationRoleColumn(); err != nil {
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

func ensureUserProgressRatingColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(user_progress);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasRating := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, "user_rating") {
			hasRating = true
			break
		}
	}
	if !hasRating {
		if _, err := DB.Exec(`ALTER TABLE user_progress ADD COLUMN user_rating REAL;`); err != nil {
			log.Printf("Warning: adding user_rating column to user_progress failed: %v", err)
		} else {
			log.Println("✓ Added user_rating column to user_progress")
		}
	}
	return nil
}

func ensureUserRoleColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(users);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasRole := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, "role") {
			hasRole = true
			break
		}
	}
	if !hasRole {
		if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'member';`); err != nil {
			log.Printf("Warning: adding role column to users failed: %v", err)
		} else {
			log.Println("✓ Added role column to users")
		}
	}
	return nil
}

func ensureUserConversationRoleColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(user_conversation_history);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasRole := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, "role") {
			hasRole = true
			break
		}
	}
	if !hasRole {
		if _, err := DB.Exec(`ALTER TABLE user_conversation_history ADD COLUMN role TEXT DEFAULT 'member';`); err != nil {
			log.Printf("Warning: adding role column to user_conversation_history failed: %v", err)
		} else {
			log.Println("✓ Added role column to user_conversation_history")
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
