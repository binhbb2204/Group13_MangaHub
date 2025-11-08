package tcp

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func HandleConnection(client *Client, manager *ClientManager, removeClient func(string)) {
	defer func() {
		log.Printf("Client disconnected: %s", client.ID)
		removeClient(client.ID)
		client.Conn.Close()
	}()

	log.Printf("Client connected: %s", client.ID)
	reader := bufio.NewReader(client.Conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading from %s: %v", client.ID, err)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		msg, err := ParseMessage([]byte(line))
		if err != nil {
			log.Printf("Error parsing message from %s: %v", client.ID, err)
			client.Conn.Write(CreateErrorMessage("Invalid message format"))
			continue
		}

		if err := routeMessage(client, msg); err != nil {
			log.Printf("Error handling message from %s: %v", client.ID, err)
			client.Conn.Write(CreateErrorMessage(err.Error()))
		}
	}
}

func routeMessage(client *Client, msg *Message) error {
	switch msg.Type {
	case "ping":
		return handlePing(client)
	case "auth":
		return handleAuth(client, msg.Payload)
	case "sync_progress":
		return handleSyncProgress(client, msg.Payload)
	case "get_library":
		return handleGetLibrary(client, msg.Payload)
	case "get_progress":
		return handleGetProgress(client, msg.Payload)
	case "add_to_library":
		return handleAddToLibrary(client, msg.Payload)
	case "remove_from_library":
		return handleRemoveFromLibrary(client, msg.Payload)
	default:
		client.Conn.Write(CreateErrorMessage("Unknown message type: " + msg.Type))
		return nil
	}
}

func handlePing(client *Client) error {
	_, err := client.Conn.Write(CreatePongMessage())
	return err
}

func handleAuth(client *Client, payload json.RawMessage) error {
	var authPayload AuthPayload
	if err := json.Unmarshal(payload, &authPayload); err != nil {
		client.Conn.Write(CreateErrorMessage("Invalid auth payload"))
		return err
	}

	if authPayload.Token == "" {
		client.Conn.Write(CreateErrorMessage("Token is required"))
		return nil
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	claims, err := utils.ValidateJWT(authPayload.Token, jwtSecret)
	if err != nil {
		client.Conn.Write(CreateErrorMessage("Invalid or expired token"))
		return nil
	}

	client.UserID = claims.UserID
	client.Username = claims.Username
	client.Authenticated = true

	log.Printf("Client %s authenticated as user %s (%s)", client.ID, client.Username, client.UserID)
	client.Conn.Write(CreateSuccessMessage("Authentication successful"))
	return nil
}

func handleSyncProgress(client *Client, payload json.RawMessage) error {
	if !client.Authenticated {
		client.Conn.Write(CreateErrorMessage("Authentication required"))
		return nil
	}

	var syncPayload SyncProgressPayload
	if err := json.Unmarshal(payload, &syncPayload); err != nil {
		client.Conn.Write(CreateErrorMessage("Invalid sync_progress payload"))
		return err
	}

	if syncPayload.MangaID == "" || syncPayload.CurrentChapter < 0 {
		client.Conn.Write(CreateErrorMessage("Invalid manga_id or current_chapter"))
		return nil
	}

	validStatuses := map[string]bool{
		"reading":      true,
		"completed":    true,
		"plan_to_read": true,
	}
	if syncPayload.Status != "" && !validStatuses[syncPayload.Status] {
		client.Conn.Write(CreateErrorMessage("Invalid status. Must be: reading, completed, or plan_to_read"))
		return nil
	}

	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)`
	err := database.DB.QueryRow(checkQuery, syncPayload.MangaID).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking manga existence: %v", err)
		client.Conn.Write(CreateErrorMessage("Database error"))
		return err
	}
	if !exists {
		client.Conn.Write(CreateErrorMessage("Manga not found"))
		return nil
	}

	now := time.Now()
	query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
              VALUES (?, ?, ?, ?, ?)
              ON CONFLICT(user_id, manga_id) DO UPDATE SET 
              current_chapter = ?, 
              status = COALESCE(?, status),
              updated_at = ?`

	status := syncPayload.Status
	if status == "" {
		status = "reading"
	}

	_, err = database.DB.Exec(query,
		client.UserID, syncPayload.MangaID, syncPayload.CurrentChapter, status, now,
		syncPayload.CurrentChapter, syncPayload.Status, now)

	if err != nil {
		log.Printf("Database error syncing progress: %v", err)
		client.Conn.Write(CreateErrorMessage("Failed to sync progress"))
		return err
	}

	log.Printf("Progress synced for user %s: MangaID=%s, Chapter=%d, Status=%s",
		client.Username, syncPayload.MangaID, syncPayload.CurrentChapter, status)

	client.Conn.Write(CreateSuccessMessage("Progress synced successfully"))
	return nil
}

func handleGetLibrary(client *Client, payload json.RawMessage) error {
	if !client.Authenticated {
		client.Conn.Write(CreateErrorMessage("Authentication required"))
		return nil
	}

	query := `
        SELECT m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url,
               up.current_chapter, up.status, up.updated_at
        FROM user_progress up
        JOIN manga m ON up.manga_id = m.id
        WHERE up.user_id = ?
        ORDER BY up.updated_at DESC
    `

	rows, err := database.DB.Query(query, client.UserID)
	if err != nil {
		log.Printf("Database error fetching library: %v", err)
		client.Conn.Write(CreateErrorMessage("Database error"))
		return err
	}
	defer rows.Close()

	type MangaProgress struct {
		MangaID        string `json:"manga_id"`
		Title          string `json:"title"`
		Author         string `json:"author"`
		Genres         string `json:"genres"`
		Status         string `json:"manga_status"`
		TotalChapters  int    `json:"total_chapters"`
		Description    string `json:"description"`
		CoverURL       string `json:"cover_url"`
		CurrentChapter int    `json:"current_chapter"`
		ReadStatus     string `json:"read_status"`
		UpdatedAt      string `json:"updated_at"`
	}

	library := []MangaProgress{}
	rowCount := 0
	for rows.Next() {
		rowCount++
		var mp MangaProgress
		var genres, description, coverURL *string
		err := rows.Scan(
			&mp.MangaID,
			&mp.Title,
			&mp.Author,
			&genres,
			&mp.Status,
			&mp.TotalChapters,
			&description,
			&coverURL,
			&mp.CurrentChapter,
			&mp.ReadStatus,
			&mp.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error scanning library row: %v", err)
			continue
		}
		if genres != nil {
			mp.Genres = *genres
		}
		if description != nil {
			mp.Description = *description
		}
		if coverURL != nil {
			mp.CoverURL = *coverURL
		}
		library = append(library, mp)
	}

	log.Printf("Fetched library for user %s: %d items (scanned %d rows)", client.Username, len(library), rowCount)
	client.Conn.Write(CreateDataMessage("library", library))
	return nil
}

func handleGetProgress(client *Client, payload json.RawMessage) error {
	if !client.Authenticated {
		client.Conn.Write(CreateErrorMessage("Authentication required"))
		return nil
	}

	var req GetProgressPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		client.Conn.Write(CreateErrorMessage("Invalid get_progress payload"))
		return err
	}

	if req.MangaID == "" {
		client.Conn.Write(CreateErrorMessage("manga_id is required"))
		return nil
	}

	var progress struct {
		CurrentChapter int    `json:"current_chapter"`
		Status         string `json:"status"`
		UpdatedAt      string `json:"updated_at"`
	}

	query := `SELECT current_chapter, status, updated_at FROM user_progress WHERE user_id = ? AND manga_id = ?`
	err := database.DB.QueryRow(query, client.UserID, req.MangaID).Scan(&progress.CurrentChapter, &progress.Status, &progress.UpdatedAt)
	if err != nil {
		client.Conn.Write(CreateErrorMessage("Progress not found"))
		return nil
	}

	client.Conn.Write(CreateDataMessage("progress", progress))
	return nil
}

func handleAddToLibrary(client *Client, payload json.RawMessage) error {
	if !client.Authenticated {
		client.Conn.Write(CreateErrorMessage("Authentication required"))
		return nil
	}

	var req AddToLibraryPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		client.Conn.Write(CreateErrorMessage("Invalid add_to_library payload"))
		return err
	}

	if req.MangaID == "" {
		client.Conn.Write(CreateErrorMessage("manga_id is required"))
		return nil
	}

	validStatuses := map[string]bool{
		"reading":      true,
		"completed":    true,
		"plan_to_read": true,
	}
	status := req.Status
	if status == "" {
		status = "plan_to_read"
	}
	if !validStatuses[status] {
		client.Conn.Write(CreateErrorMessage("Invalid status. Must be: reading, completed, or plan_to_read"))
		return nil
	}

	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)`
	err := database.DB.QueryRow(checkQuery, req.MangaID).Scan(&exists)
	if err != nil || !exists {
		client.Conn.Write(CreateErrorMessage("Manga not found"))
		return nil
	}

	now := time.Now()
	query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
              VALUES (?, ?, 0, ?, ?)
              ON CONFLICT(user_id, manga_id) DO UPDATE SET status = ?, updated_at = ?`

	_, err = database.DB.Exec(query, client.UserID, req.MangaID, status, now, status, now)
	if err != nil {
		log.Printf("Database error adding to library: %v", err)
		client.Conn.Write(CreateErrorMessage("Failed to add manga to library"))
		return err
	}

	log.Printf("User %s added manga %s to library with status %s", client.Username, req.MangaID, status)
	client.Conn.Write(CreateSuccessMessage("Manga added to library successfully"))
	return nil
}

func handleRemoveFromLibrary(client *Client, payload json.RawMessage) error {
	if !client.Authenticated {
		client.Conn.Write(CreateErrorMessage("Authentication required"))
		return nil
	}

	var req RemoveFromLibraryPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		client.Conn.Write(CreateErrorMessage("Invalid remove_from_library payload"))
		return err
	}

	if req.MangaID == "" {
		client.Conn.Write(CreateErrorMessage("manga_id is required"))
		return nil
	}

	query := `DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?`
	result, err := database.DB.Exec(query, client.UserID, req.MangaID)
	if err != nil {
		log.Printf("Database error removing from library: %v", err)
		client.Conn.Write(CreateErrorMessage("Failed to remove manga from library"))
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		client.Conn.Write(CreateErrorMessage("Manga not in library"))
		return nil
	}

	log.Printf("User %s removed manga %s from library", client.Username, req.MangaID)
	client.Conn.Write(CreateSuccessMessage("Manga removed from library successfully"))
	return nil
}
