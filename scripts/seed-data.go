package main

import (
	"log"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
)

func main() {
	if err := database.InitDatabase("data/mangahub.db"); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	_, err := database.DB.Exec(`INSERT OR IGNORE INTO manga (id, title, author, genres, status, total_chapters, description) VALUES 
		('one-piece', 'One Piece', 'Oda Eiichiro', '["Action","Adventure","Shounen"]', 'ongoing', 1100, 'A young pirate''s adventure to become the Pirate King'),
		('naruto', 'Naruto', 'Kishimoto Masashi', '["Action","Shounen"]', 'completed', 700, 'A young ninja''s journey to become Hokage')`)
	if err != nil {
		log.Fatalf("Failed to insert manga: %v", err)
	}

	_, err = database.DB.Exec(`DELETE FROM users WHERE id = 'user123'`)
	if err != nil {
		log.Printf("Note: Could not delete existing user: %v", err)
	}

	result, err := database.DB.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES ('user123', 'testuser', '$2a$10$dummy', CURRENT_TIMESTAMP)`)
	if err != nil {
		log.Fatalf("Failed to insert user: %v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("Inserted %d user(s)", rows)

	log.Println("Test data inserted successfully")
}
