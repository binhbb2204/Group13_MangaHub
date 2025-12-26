package user

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	manga "github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-gonic/gin"
)

// Handler handles user-related operations
type Handler struct {
	bridge         *bridge.Bridge
	externalSource manga.ExternalSource
}

// NewHandler creates a new user handler
func NewHandler(br *bridge.Bridge) *Handler {
	return &Handler{
		bridge:         br,
		externalSource: manga.NewMALSource(),
	}
}

// NewHandlerWithSource creates a new user handler with custom external source (for testing)
func NewHandlerWithSource(br *bridge.Bridge, source manga.ExternalSource) *Handler {
	return &Handler{
		bridge:         br,
		externalSource: source,
	}
}

// resolveTCPAddr determines the TCP server address using env overrides or auto-detected local IP.
// Order of precedence:
// 1) TCP_HOST env var (explicit override)
// 2) Auto-detected local IPv4 via utils.GetLocalIP()
// Port defaults to TCP_PORT env or 9090.
func resolveTCPAddr() string {
	tcpPort := os.Getenv("TCP_PORT")
	if tcpPort == "" {
		tcpPort = "9090"
	}
	tcpHost := os.Getenv("TCP_HOST")
	if tcpHost == "" {
		tcpHost = utils.GetLocalIP()
	}
	return net.JoinHostPort(tcpHost, tcpPort)
}

// resolveUDPAddr determines the UDP server address using env overrides or auto-detected local IP.
// Order of precedence:
// 1) UDP_HOST env var (explicit override)
// 2) Auto-detected local IPv4 via utils.GetLocalIP()
// Port defaults to UDP_PORT env or 9091.
func resolveUDPAddr() string {
	udpPort := os.Getenv("UDP_PORT")
	if udpPort == "" {
		udpPort = "9091"
	}
	udpHost := os.Getenv("UDP_HOST")
	if udpHost == "" {
		udpHost = utils.GetLocalIP()
	}
	return net.JoinHostPort(udpHost, udpPort)
}

// GetProfile gets the current user's profile
func (h *Handler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user models.User
	query := `SELECT id, username, email, created_at FROM users WHERE id = ?`
	err := database.DB.QueryRow(query, userID).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// AddToLibrary adds a manga to user's library
func (h *Handler) AddToLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.AddToLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// DEBUG: Log that we're entering this function
	log.Printf("[DEBUG] AddToLibrary called for manga ID: %s, user ID: %s", req.MangaID, userID)

	// Fetch complete manga metadata from MAL API
	ctx := context.Background()
	mangaData, err := h.externalSource.GetMangaByID(ctx, req.MangaID)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch manga from MAL for ID %s: %v", req.MangaID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found in external API"})
		return
	}
	log.Printf("[DEBUG] Successfully fetched manga from MAL: %s (Total Chapters: %d)", mangaData.Title, mangaData.TotalChapters)

	// MangaDex lookup removed

	// Save or update manga in database with complete metadata (UPSERT)
	if err := h.saveMangaToDB(mangaData); err != nil {
		log.Printf("[ERROR] Failed to save manga %s to database: %v", req.MangaID, err)
		// Continue anyway - user progress is more important
	}

	// Determine strict/non-strict TCP forwarding
	strictTCPForward := os.Getenv("TCP_FORWARD_REQUIRED") == "true"
	tcpForwardEnabled := os.Getenv("TCP_FORWARD_ENABLED") == "true"

	// Determine strict/non-strict UDP forwarding
	strictUDPForward := os.Getenv("UDP_FORWARD_REQUIRED") == "true"
	udpForwardEnabled := os.Getenv("UDP_FORWARD_ENABLED") == "true"

	// Auto-set status: new manga always starts as "plan_to_read"
	status := "plan_to_read"

	if strictTCPForward {
		// Strict mode: require TCP server to process the add_to_library operation
		tcpAddr := resolveTCPAddr()
		token := c.GetHeader("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")

		if err := forwardAddToLibraryToTCP(tcpAddr, token, req.MangaID, status); err != nil {
			log.Printf("[ERROR] TCP add_to_library required but failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TCP forward failed; TCP server unavailable"})
			return
		}
	} else {
		// Local DB write remains the source of truth (non-strict)
		query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
				  VALUES (?, ?, 0, ?, ?)
				  ON CONFLICT(user_id, manga_id) DO UPDATE SET status = ?, updated_at = ?`

		now := time.Now()
		if _, err = database.DB.Exec(query, userID, req.MangaID, status, now, status, now); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add manga to library"})
			return
		}

		// Non-strict forward (optional)
		if tcpForwardEnabled {
			tcpAddr := resolveTCPAddr()
			token := c.GetHeader("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
			go func() {
				if err := forwardAddToLibraryToTCP(tcpAddr, token, req.MangaID, status); err != nil {
					log.Printf("[WARN] TCP add_to_library forward failed: %v", err)
				}
			}()
		}
	}

	// UDP forwarding logic
	if strictUDPForward {
		// Strict mode: require UDP server to process the add_to_library notification
		udpAddr := resolveUDPAddr()
		if err := forwardAddToLibraryToUDP(udpAddr, userID, req.MangaID, status); err != nil {
			log.Printf("[ERROR] UDP add_to_library required but failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "UDP forward failed; UDP server unavailable"})
			return
		}
	} else if udpForwardEnabled {
		// Non-strict forward (optional)
		udpAddr := resolveUDPAddr()
		go func() {
			if err := forwardAddToLibraryToUDP(udpAddr, userID, req.MangaID, status); err != nil {
				log.Printf("[WARN] UDP add_to_library forward failed: %v", err)
			}
		}()
	}

	// Notify locally (bridge may be no-op in standalone API mode)
	h.bridge.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{UserID: userID, MangaID: req.MangaID, Action: "added"})

	c.JSON(http.StatusOK, gin.H{"message": "Manga added to library successfully"})
}

// saveMangaToDB saves or updates manga in database with UPSERT
func (h *Handler) saveMangaToDB(m *models.Manga) error {
	genresJSON, err := json.Marshal(m.Genres)
	if err != nil {
		return err
	}

	query := `INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, media_type)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	          ON CONFLICT(id) DO UPDATE SET 
	              title = excluded.title,
	              author = excluded.author,
	              genres = excluded.genres,
	              status = excluded.status,
	              total_chapters = excluded.total_chapters,
	              description = excluded.description,
	              cover_url = excluded.cover_url,
	              media_type = excluded.media_type`

	_, err = database.DB.Exec(
		query,
		m.ID,
		m.Title,
		m.Author,
		string(genresJSON),
		m.Status,
		m.TotalChapters,
		m.Description,
		m.CoverURL,
		m.MediaType,
	)

	return err
}

// GetLibrary gets user's manga library
func (h *Handler) GetLibrary(c *gin.Context) {
	userID := c.GetString("user_id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	query := `
		SELECT m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url,
		       up.current_chapter, up.status, up.user_rating, up.updated_at
		FROM user_progress up
		JOIN manga m ON up.manga_id = m.id
		WHERE up.user_id = ?
		ORDER BY up.updated_at DESC
	`

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	library := models.UserLibrary{
		Reading:    []models.MangaProgress{},
		Completed:  []models.MangaProgress{},
		PlanToRead: []models.MangaProgress{},
	}

	for rows.Next() {
		var mp models.MangaProgress
		var genresJSON string
		var userRating sql.NullFloat64 // Handle NULL values

		err := rows.Scan(
			&mp.Manga.ID,
			&mp.Manga.Title,
			&mp.Manga.Author,
			&genresJSON,
			&mp.Manga.Status,
			&mp.Manga.TotalChapters,
			&mp.Manga.Description,
			&mp.Manga.CoverURL,
			&mp.CurrentChapter,
			&mp.Status,
			&userRating, // Scan into sql.NullFloat64
			&mp.UpdatedAt,
		)
		if err != nil {
			// Log error but continue processing other rows
			log.Printf("[ERROR] Failed to scan library row: %v", err)
			continue
		}

		// Convert user rating (use pointer for explicit null)
		if userRating.Valid {
			rating := userRating.Float64
			mp.UserRating = &rating
		} else {
			mp.UserRating = nil // Explicit null in JSON
		}

		// Parse genres JSON
		if genresJSON != "" {
			json.Unmarshal([]byte(genresJSON), &mp.Manga.Genres)
		}

		// Categorize by status
		switch mp.Status {
		case "reading":
			library.Reading = append(library.Reading, mp)
		case "completed":
			library.Completed = append(library.Completed, mp)
		case "plan_to_read":
			library.PlanToRead = append(library.PlanToRead, mp)
		}
	}

	c.JSON(http.StatusOK, library)
}

// UpdateProgress updates user's reading progress
func (h *Handler) UpdateProgress(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if manga exists in user's library
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM user_progress WHERE user_id = ? AND manga_id = ?)`
	err := database.DB.QueryRow(checkQuery, userID, req.MangaID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not in library"})
		return
	}

	// Get manga total_chapters for auto-status calculation
	var totalChapters int
	mangaQuery := `SELECT total_chapters FROM manga WHERE id = ?`
	database.DB.QueryRow(mangaQuery, req.MangaID).Scan(&totalChapters)

	// Build update query - start with updated_at
	query := `UPDATE user_progress SET updated_at = ?`
	args := []interface{}{time.Now()}

	// Auto-calculate status based on chapter progress
	var autoStatus string
	if req.CurrentChapter != nil {
		currentChapter := *req.CurrentChapter

		if currentChapter == 0 {
			autoStatus = "plan_to_read"
		} else if totalChapters > 0 && currentChapter >= totalChapters {
			autoStatus = "completed"
		} else {
			autoStatus = "reading"
		}

		query += `, current_chapter = ?, status = ?`
		args = append(args, currentChapter, autoStatus)
	} else if req.Status != "" {
		// Allow manual status override if no chapter update
		query += `, status = ?`
		args = append(args, req.Status)
	}

	if req.UserRating != nil {
		query += `, user_rating = ?`
		args = append(args, *req.UserRating)
	}

	query += ` WHERE user_id = ? AND manga_id = ?`
	args = append(args, userID, req.MangaID)

	_, err = database.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
		return
	}

	// Optional/Required: forward to standalone TCP server when running separately.
	// Enable with TCP_FORWARD_ENABLED=true. Enforce strict failure with TCP_FORWARD_REQUIRED=true.
	strictTCPForward := os.Getenv("TCP_FORWARD_REQUIRED") == "true"
	tcpForwardEnabled := os.Getenv("TCP_FORWARD_ENABLED") == "true"

	// Optional/Required: forward to standalone UDP server when running separately.
	// Enable with UDP_FORWARD_ENABLED=true. Enforce strict failure with UDP_FORWARD_REQUIRED=true.
	strictUDPForward := os.Getenv("UDP_FORWARD_REQUIRED") == "true"
	udpForwardEnabled := os.Getenv("UDP_FORWARD_ENABLED") == "true"

	if strictTCPForward || tcpForwardEnabled {
		tcpAddr := resolveTCPAddr()

		// Derive token from Authorization header (Bearer <token>)
		token := c.GetHeader("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")

		forwardStatus := autoStatus
		if forwardStatus == "" && req.Status != "" {
			forwardStatus = req.Status
		}
		if forwardStatus == "" {
			forwardStatus = "reading"
		}

		chapterVal := 0
		if req.CurrentChapter != nil {
			chapterVal = *req.CurrentChapter
		}

		if strictTCPForward {
			// In strict mode, fail the request if TCP forward fails
			if err := forwardProgressToTCP(tcpAddr, token, userID, req.MangaID, chapterVal, forwardStatus); err != nil {
				log.Printf("[ERROR] TCP forward required but failed: %v", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TCP forward failed; TCP server unavailable"})
				return
			}
		} else {
			// Non-strict: fire-and-forget
			go func() {
				if err := forwardProgressToTCP(tcpAddr, token, userID, req.MangaID, chapterVal, forwardStatus); err != nil {
					log.Printf("[WARN] TCP forward failed: %v", err)
				}
			}()
		}
	}

	// UDP forwarding logic
	if strictUDPForward || udpForwardEnabled {
		udpAddr := resolveUDPAddr()

		forwardStatus := autoStatus
		if forwardStatus == "" && req.Status != "" {
			forwardStatus = req.Status
		}
		if forwardStatus == "" {
			forwardStatus = "reading"
		}

		chapterVal := 0
		if req.CurrentChapter != nil {
			chapterVal = *req.CurrentChapter
		}

		userRating := 0.0
		if req.UserRating != nil {
			userRating = *req.UserRating
		}

		if strictUDPForward {
			// In strict mode, fail the request if UDP forward fails
			if err := forwardProgressToUDP(udpAddr, userID, req.MangaID, chapterVal, forwardStatus, userRating); err != nil {
				log.Printf("[ERROR] UDP forward required but failed: %v", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "UDP forward failed; UDP server unavailable"})
				return
			}
		} else {
			// Non-strict: fire-and-forget
			go func() {
				if err := forwardProgressToUDP(udpAddr, userID, req.MangaID, chapterVal, forwardStatus, userRating); err != nil {
					log.Printf("[WARN] UDP forward failed: %v", err)
				}
			}()
		}
	}

	h.bridge.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
		UserID:  userID,
		MangaID: req.MangaID,
		ChapterID: func() int {
			if req.CurrentChapter != nil {
				return *req.CurrentChapter
			}
			return 0
		}(),
		Status:       autoStatus,
		LastReadDate: time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Progress updated successfully"})
}

// forwardProgressToTCP opens a short-lived TCP connection to the TCP server and
// sends auth + sync_progress messages using the existing TCP JSON protocol.
func forwardProgressToTCP(tcpAddr, jwtToken, userID, mangaID string, chapter int, status string) error {
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial tcp: %w", err)
	}
	defer conn.Close()

	w := bufio.NewWriter(conn)

	// auth
	if _, err := fmt.Fprintf(w, `{"type":"auth","payload":{"token":"%s"}}`+"\n", jwtToken); err != nil {
		return fmt.Errorf("write auth: %w", err)
	}

	// sync_progress
	if _, err := fmt.Fprintf(w,
		`{"type":"sync_progress","payload":{"user_id":"%s","manga_id":"%s","current_chapter":%d,"status":"%s"}}`+"\n",
		userID, mangaID, chapter, status); err != nil {
		return fmt.Errorf("write sync_progress: %w", err)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

// ForwardProgressTest: debug endpoint to manually trigger TCP forward without bridge
// Requires auth; body: {"manga_id": "...", "chapter": 5, "status": "reading"}
func (h *Handler) ForwardProgressTest(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var body struct {
		MangaID string `json:"manga_id"`
		Chapter int    `json:"chapter"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.MangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "manga_id and chapter required"})
		return
	}

	tcpAddr := resolveTCPAddr()

	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	status := body.Status
	if status == "" {
		status = "reading"
	}

	if err := forwardProgressToTCP(tcpAddr, token, userID, body.MangaID, body.Chapter, status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetProgress gets user's progress for a specific manga
func (h *Handler) GetProgress(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	mangaID := c.Param("manga_id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Manga ID is required"})
		return
	}

	query := `
		SELECT current_chapter, status, user_rating, updated_at
		FROM user_progress
		WHERE user_id = ? AND manga_id = ?
	`

	var currentChapter int
	var status string
	var userRating sql.NullFloat64
	var updatedAt time.Time

	err := database.DB.QueryRow(query, userID, mangaID).Scan(
		&currentChapter,
		&status,
		&userRating,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Manga not in library"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	response := gin.H{
		"manga_id":        mangaID,
		"current_chapter": currentChapter,
		"status":          status,
		"updated_at":      updatedAt,
	}

	// Add user_rating if it exists
	if userRating.Valid {
		response["user_rating"] = userRating.Float64
	} else {
		response["user_rating"] = nil
	}

	c.JSON(http.StatusOK, response)
}

// RemoveFromLibrary removes manga from user's library
func (h *Handler) RemoveFromLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	mangaID := c.Param("manga_id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Manga ID is required"})
		return
	}

	strictTCPForward := os.Getenv("TCP_FORWARD_REQUIRED") == "true"
	tcpForwardEnabled := os.Getenv("TCP_FORWARD_ENABLED") == "true"

	strictUDPForward := os.Getenv("UDP_FORWARD_REQUIRED") == "true"
	udpForwardEnabled := os.Getenv("UDP_FORWARD_ENABLED") == "true"

	if strictTCPForward {
		// Require TCP to remove
		tcpAddr := resolveTCPAddr()
		token := c.GetHeader("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
		if err := forwardRemoveFromLibraryToTCP(tcpAddr, token, mangaID); err != nil {
			log.Printf("[ERROR] TCP remove_from_library required but failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "TCP forward failed; TCP server unavailable"})
			return
		}
	} else {
		// Local DB removal (non-strict)
		query := `DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?`
		result, err := database.DB.Exec(query, userID, mangaID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove manga"})
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Manga not in library"})
			return
		}
		if tcpForwardEnabled {
			tcpAddr := resolveTCPAddr()
			token := c.GetHeader("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
			go func() {
				if err := forwardRemoveFromLibraryToTCP(tcpAddr, token, mangaID); err != nil {
					log.Printf("[WARN] TCP remove_from_library forward failed: %v", err)
				}
			}()
		}
	}

	// UDP forwarding logic
	if strictUDPForward {
		// Require UDP to remove
		udpAddr := resolveUDPAddr()
		if err := forwardRemoveFromLibraryToUDP(udpAddr, userID, mangaID); err != nil {
			log.Printf("[ERROR] UDP remove_from_library required but failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "UDP forward failed; UDP server unavailable"})
			return
		}
	} else if udpForwardEnabled {
		// Non-strict forward (optional)
		udpAddr := resolveUDPAddr()
		go func() {
			if err := forwardRemoveFromLibraryToUDP(udpAddr, userID, mangaID); err != nil {
				log.Printf("[WARN] UDP remove_from_library forward failed: %v", err)
			}
		}()
	}

	h.bridge.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{
		UserID:  userID,
		MangaID: mangaID,
		Action:  "removed",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Manga removed from library successfully"})
}

// forwardAddToLibraryToTCP sends auth + add_to_library
func forwardAddToLibraryToTCP(tcpAddr, jwtToken, mangaID, status string) error {
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial tcp: %w", err)
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	if _, err := fmt.Fprintf(w, `{"type":"auth","payload":{"token":"%s"}}`+"\n", jwtToken); err != nil {
		return fmt.Errorf("write auth: %w", err)
	}
	if _, err := fmt.Fprintf(w, `{"type":"add_to_library","payload":{"manga_id":"%s","status":"%s"}}`+"\n", mangaID, status); err != nil {
		return fmt.Errorf("write add_to_library: %w", err)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

// forwardRemoveFromLibraryToTCP sends auth + remove_from_library
func forwardRemoveFromLibraryToTCP(tcpAddr, jwtToken, mangaID string) error {
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial tcp: %w", err)
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	if _, err := fmt.Fprintf(w, `{"type":"auth","payload":{"token":"%s"}}`+"\n", jwtToken); err != nil {
		return fmt.Errorf("write auth: %w", err)
	}
	if _, err := fmt.Fprintf(w, `{"type":"remove_from_library","payload":{"manga_id":"%s"}}`+"\n", mangaID); err != nil {
		return fmt.Errorf("write remove_from_library: %w", err)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

// forwardProgressToUDP sends a UDP notification about progress update
func forwardProgressToUDP(udpAddr, userID, mangaID string, chapter int, status string, rating float64) error {
	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("resolve udp addr: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	// Create notification message
	data := map[string]interface{}{
		"manga_id": mangaID,
		"chapter":  chapter,
		"status":   status,
	}
	if rating > 0 {
		data["rating"] = rating
	}

	msg := map[string]interface{}{
		"type":       "notification",
		"event_type": "progress_update",
		"user_id":    userID,
		"data":       data,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		return fmt.Errorf("write udp: %w", err)
	}

	return nil
}

// forwardAddToLibraryToUDP sends a UDP notification about add to library
func forwardAddToLibraryToUDP(udpAddr, userID, mangaID, status string) error {
	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("resolve udp addr: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	// Create notification message
	msg := map[string]interface{}{
		"type":       "notification",
		"event_type": "library_update",
		"user_id":    userID,
		"data": map[string]interface{}{
			"manga_id": mangaID,
			"status":   status,
			"action":   "add",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		return fmt.Errorf("write udp: %w", err)
	}

	return nil
}

// forwardRemoveFromLibraryToUDP sends a UDP notification about remove from library
func forwardRemoveFromLibraryToUDP(udpAddr, userID, mangaID string) error {
	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("resolve udp addr: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	// Create notification message
	msg := map[string]interface{}{
		"type":       "notification",
		"event_type": "library_update",
		"user_id":    userID,
		"data": map[string]interface{}{
			"manga_id": mangaID,
			"action":   "remove",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		return fmt.Errorf("write udp: %w", err)
	}

	return nil
}

// SyncConnect handler: Connect to TCP sync server
func (h *Handler) SyncConnect(c *gin.Context) {
	var req struct {
		DeviceType string `json:"device_type"`
		DeviceName string `json:"device_name"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	// Get token from header
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(401, gin.H{"error": "missing authorization token"})
		return
	}
	// Remove "Bearer " prefix
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	tcpAddr := resolveTCPAddr()
	conn, err := net.DialTimeout("tcp", tcpAddr, 10*time.Second)
	if err != nil {
		c.JSON(503, gin.H{"error": "failed to connect to sync server", "details": fmt.Sprintf("TCP server at %s is unreachable", tcpAddr)})
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Send auth
	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"token": token,
		},
	}
	authJSON, _ := json.Marshal(authMsg)
	authJSON = append(authJSON, '\n')

	if _, err := conn.Write(authJSON); err != nil {
		c.JSON(503, gin.H{"error": "failed to authenticate with sync server"})
		return
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		c.JSON(503, gin.H{"error": "failed to read auth response"})
		return
	}

	var authResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &authResponse); err != nil {
		c.JSON(503, gin.H{"error": "invalid auth response"})
		return
	}

	if authResponse["type"] == "error" {
		c.JSON(401, gin.H{"error": "authentication failed"})
		return
	}

	// Send connect
	if req.DeviceType == "" {
		req.DeviceType = "web"
	}
	if req.DeviceName == "" {
		req.DeviceName = "web-client"
	}

	connectMsg := map[string]interface{}{
		"type": "connect",
		"payload": map[string]string{
			"device_type": req.DeviceType,
			"device_name": req.DeviceName,
		},
	}
	connectJSON, _ := json.Marshal(connectMsg)
	connectJSON = append(connectJSON, '\n')

	if _, err := conn.Write(connectJSON); err != nil {
		c.JSON(503, gin.H{"error": "failed to send connect message"})
		return
	}

	response, err = reader.ReadString('\n')
	if err != nil {
		c.JSON(503, gin.H{"error": "failed to read connect response"})
		return
	}

	var connectResponse struct {
		Type    string `json:"type"`
		Payload struct {
			SessionID   string `json:"session_id"`
			ConnectedAt string `json:"connected_at"`
		} `json:"payload"`
	}

	if err := json.Unmarshal([]byte(response), &connectResponse); err != nil {
		c.JSON(503, gin.H{"error": "invalid connect response"})
		return
	}

	c.JSON(200, gin.H{
		"status":         "connected",
		"session_id":     connectResponse.Payload.SessionID,
		"server_address": tcpAddr,
		"connected_at":   connectResponse.Payload.ConnectedAt,
	})
}

// SyncGetStatus handler: Get sync server status
func (h *Handler) SyncGetStatus(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(401, gin.H{"error": "missing authorization token"})
		return
	}
	// Remove "Bearer " prefix
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	tcpAddr := resolveTCPAddr()
	conn, err := net.DialTimeout("tcp", tcpAddr, 5*time.Second)
	if err != nil {
		c.JSON(503, gin.H{
			"type": "status_response",
			"payload": gin.H{
				"connection_status": "disconnected",
				"error":             fmt.Sprintf("TCP server at %s is unreachable", tcpAddr),
			},
		})
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Send auth
	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"token": token,
		},
	}
	authJSON, _ := json.Marshal(authMsg)
	authJSON = append(authJSON, '\n')

	if _, err := conn.Write(authJSON); err != nil {
		c.JSON(503, gin.H{"error": "failed to authenticate", "status": "error"})
		return
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		c.JSON(503, gin.H{"error": "failed to read auth response", "status": "error"})
		return
	}

	var authResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &authResponse); err != nil {
		c.JSON(503, gin.H{"error": "invalid auth response", "status": "error"})
		return
	}

	if authResponse["type"] == "error" {
		c.JSON(401, gin.H{"error": "authentication failed", "status": "error"})
		return
	}

	// Send connect
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "web-status-query"
	}
	connectMsg := map[string]interface{}{
		"type": "connect",
		"payload": map[string]string{
			"device_type": "web",
			"device_name": hostname,
		},
	}
	connectJSON, _ := json.Marshal(connectMsg)
	connectJSON = append(connectJSON, '\n')

	if _, err := conn.Write(connectJSON); err != nil {
		c.JSON(503, gin.H{"error": "failed to send connect", "status": "error"})
		return
	}

	response, err = reader.ReadString('\n')
	if err != nil {
		c.JSON(503, gin.H{"error": "failed to read connect response", "status": "error"})
		return
	}

	// Send status request
	statusMsg := map[string]interface{}{
		"type":    "status_request",
		"payload": map[string]interface{}{},
	}
	statusJSON, _ := json.Marshal(statusMsg)
	statusJSON = append(statusJSON, '\n')

	if _, err := conn.Write(statusJSON); err != nil {
		c.JSON(503, gin.H{"error": "failed to send status request", "status": "error"})
		return
	}

	response, err = reader.ReadString('\n')
	if err != nil {
		c.JSON(503, gin.H{"error": "failed to read status response", "status": "error"})
		return
	}

	var statusResp map[string]interface{}
	if err := json.Unmarshal([]byte(response), &statusResp); err != nil {
		c.JSON(503, gin.H{"error": "invalid status response", "status": "error"})
		return
	}

	c.JSON(200, statusResp)
}

// SyncDisconnect handler: Disconnect from TCP sync server
func (h *Handler) SyncDisconnect(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(401, gin.H{"error": "missing authorization token"})
		return
	}
	// Remove "Bearer " prefix
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	tcpAddr := resolveTCPAddr()
	conn, err := net.DialTimeout("tcp", tcpAddr, 5*time.Second)
	if err != nil {
		// If TCP server is not reachable, still return success (already disconnected)
		c.JSON(200, gin.H{"status": "disconnected"})
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Send auth
	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"token": token,
		},
	}
	authJSON, _ := json.Marshal(authMsg)
	authJSON = append(authJSON, '\n')

	conn.Write(authJSON)
	reader.ReadString('\n') // Read auth response

	// Send disconnect
	disconnectMsg := map[string]interface{}{
		"type":    "disconnect",
		"payload": map[string]string{},
	}
	disconnectJSON, _ := json.Marshal(disconnectMsg)
	disconnectJSON = append(disconnectJSON, '\n')

	conn.Write(disconnectJSON)

	c.JSON(200, gin.H{"status": "disconnected"})
}
