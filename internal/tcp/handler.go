package tcp

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func HandleConnection(client *Client, manager *ClientManager, removeClient func(string), br *bridge.Bridge, sessionMgr *SessionManager, heartbeatMgr *HeartbeatManager) {
	log := logger.WithFields(map[string]interface{}{
		"client_id": client.ID,
		"component": "tcp_handler",
	})

	defer func() {
		if client.UserID != "" && br != nil {
			br.UnregisterTCPClient(client.Conn, client.UserID)
		}
		sessionMgr.RemoveSessionByClientID(client.ID)
		log.Info("client_disconnected")
		removeClient(client.ID)
		client.Conn.Close()
	}()

	log.Info("client_connected")
	reader := bufio.NewReader(client.Conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			netErr := NewNetworkReadError(err)
			log.Error("connection_read_error", "error", netErr.Error())
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		msg, err := ParseMessage([]byte(line))
		if err != nil {
			protoErr := NewProtocolInvalidFormatError(err)
			log.Warn("message_parse_error", "error", protoErr.Error(), "raw_message", line)
			SendError(client, protoErr)
			continue
		}

		// Increment messages received counter
		if session, ok := sessionMgr.GetSessionByClientID(client.ID); ok {
			sessionMgr.IncrementMessagesReceived(session.SessionID)
		}

		if err := routeMessage(client, msg, log, br, sessionMgr, heartbeatMgr); err != nil {
			log.Error("message_handling_error",
				"error", err.Error(),
				"message_type", msg.Type)
			SendError(client, err)
		}

		if session, ok := sessionMgr.GetSessionByClientID(client.ID); ok {
			sessionMgr.IncrementMessagesSent(session.SessionID)
		}
	}
}

func routeMessage(client *Client, msg *Message, log *logger.Logger, br *bridge.Bridge, sessionMgr *SessionManager, heartbeatMgr *HeartbeatManager) error {
	log = log.WithContext("message_type", msg.Type)

	switch msg.Type {
	case "ping":
		return handlePing(client, log)
	case "auth":
		return handleAuth(client, msg.Payload, log, br)
	case "connect":
		return handleConnect(client, msg.Payload, log, sessionMgr, heartbeatMgr)
	case "disconnect":
		return handleDisconnect(client, msg.Payload, log, sessionMgr)
	case "heartbeat":
		return handleHeartbeat(client, msg.Payload, log, heartbeatMgr)
	case "status_request":
		return handleStatusRequest(client, log, sessionMgr, heartbeatMgr)
	case "subscribe_updates":
		return handleSubscribeUpdates(client, msg.Payload, log, sessionMgr)
	case "unsubscribe_updates":
		return handleUnsubscribeUpdates(client, log, sessionMgr)
	case "sync_progress":
		return handleSyncProgress(client, msg.Payload, log, br, sessionMgr)
	case "get_library":
		return handleGetLibrary(client, msg.Payload, log)
	case "get_progress":
		return handleGetProgress(client, msg.Payload, log)
	case "add_to_library":
		return handleAddToLibrary(client, msg.Payload, log, br)
	case "remove_from_library":
		return handleRemoveFromLibrary(client, msg.Payload, log, br)
	default:
		err := NewProtocolUnknownTypeError(msg.Type)
		SendError(client, err)
		return nil
	}
}

func handlePing(client *Client, log *logger.Logger) error {
	log.Debug("ping_received")
	_, err := client.Conn.Write(CreatePongMessage())
	if err != nil {
		return NewNetworkWriteError(err)
	}
	return nil
}

func handleAuth(client *Client, payload json.RawMessage, log *logger.Logger, br *bridge.Bridge) error {
	var authPayload AuthPayload
	if err := json.Unmarshal(payload, &authPayload); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid auth payload")
		SendError(client, protoErr)
		return protoErr
	}

	if authPayload.Token == "" {
		authErr := NewAuthTokenMissingError()
		SendError(client, authErr)
		return authErr
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	claims, err := utils.ValidateJWT(authPayload.Token, jwtSecret)
	if err != nil {
		authErr := NewAuthTokenInvalidError()
		log.Warn("authentication_failed", "error", err.Error())
		SendError(client, authErr)
		return authErr
	}

	client.UserID = claims.UserID
	client.Username = claims.Username
	client.Authenticated = true

	if br != nil {
		br.RegisterTCPClient(client.Conn, client.UserID)
	}

	log.Info("client_authenticated",
		"user_id", client.UserID,
		"username", client.Username)
	client.Conn.Write(CreateSuccessMessage("Authentication successful"))
	return nil
}

func handleSyncProgress(client *Client, payload json.RawMessage, log *logger.Logger, br *bridge.Bridge, sessionMgr *SessionManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	log = log.WithFields(map[string]interface{}{
		"user_id":  client.UserID,
		"username": client.Username,
	})

	var syncPayload SyncProgressPayload
	if err := json.Unmarshal(payload, &syncPayload); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid sync_progress payload")
		SendError(client, protoErr)
		return protoErr
	}

	if syncPayload.MangaID == "" || syncPayload.CurrentChapter < 0 {
		bizErr := NewBizInvalidMangaIDError()
		if syncPayload.CurrentChapter < 0 {
			bizErr = NewBizInvalidChapterError(syncPayload.CurrentChapter)
		}
		SendError(client, bizErr)
		return bizErr
	}

	validStatuses := map[string]bool{
		"reading":      true,
		"completed":    true,
		"plan_to_read": true,
	}
	if syncPayload.Status != "" && !validStatuses[syncPayload.Status] {
		bizErr := NewBizInvalidStatusError(syncPayload.Status)
		SendError(client, bizErr)
		return bizErr
	}

	var exists bool
	var mangaTitle string
	checkQuery := `SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?), COALESCE((SELECT title FROM manga WHERE id = ?), '')`
	err := database.DB.QueryRow(checkQuery, syncPayload.MangaID, syncPayload.MangaID).Scan(&exists, &mangaTitle)
	if err != nil {
		dbErr := NewDatabaseQueryError(err)
		log.Error("database_error_checking_manga", "error", err.Error(), "manga_id", syncPayload.MangaID)
		SendError(client, dbErr)
		return dbErr
	}
	if !exists {
		bizErr := NewBizMangaNotFoundError(syncPayload.MangaID)
		SendError(client, bizErr)
		return bizErr
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
		dbErr := NewDatabaseQueryError(err)
		log.Error("database_error_syncing_progress", "error", err.Error())
		SendError(client, dbErr)
		return dbErr
	}

	log.Info("progress_synced",
		"manga_id", syncPayload.MangaID,
		"chapter", syncPayload.CurrentChapter,
		"status", status)

	if session, ok := sessionMgr.GetSessionByClientID(client.ID); ok {
		sessionMgr.UpdateLastSyncWithTitle(session.SessionID, syncPayload.MangaID, mangaTitle, syncPayload.CurrentChapter)
	}

	if br != nil {
		br.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
			UserID:       client.UserID,
			MangaID:      syncPayload.MangaID,
			ChapterID:    syncPayload.CurrentChapter,
			Status:       status,
			LastReadDate: now,
		})
	}

	client.Conn.Write(CreateSuccessMessage("Progress synced successfully"))
	return nil
}

func handleGetLibrary(client *Client, payload json.RawMessage, log *logger.Logger) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	log = log.WithFields(map[string]interface{}{
		"user_id":  client.UserID,
		"username": client.Username,
	})

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
		dbErr := NewDatabaseQueryError(err)
		log.Error("database_error_fetching_library", "error", err.Error())
		SendError(client, dbErr)
		return dbErr
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
			log.Warn("error_scanning_library_row", "error", err.Error())
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

	log.Info("library_fetched",
		"item_count", len(library),
		"rows_scanned", rowCount)
	client.Conn.Write(CreateDataMessage("library", library))
	return nil
}

func handleGetProgress(client *Client, payload json.RawMessage, log *logger.Logger) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	log = log.WithFields(map[string]interface{}{
		"user_id":  client.UserID,
		"username": client.Username,
	})

	var req GetProgressPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid get_progress payload")
		SendError(client, protoErr)
		return protoErr
	}

	if req.MangaID == "" {
		bizErr := NewBizInvalidMangaIDError()
		SendError(client, bizErr)
		return bizErr
	}

	var progress struct {
		CurrentChapter int    `json:"current_chapter"`
		Status         string `json:"status"`
		UpdatedAt      string `json:"updated_at"`
	}

	query := `SELECT current_chapter, status, updated_at FROM user_progress WHERE user_id = ? AND manga_id = ?`
	err := database.DB.QueryRow(query, client.UserID, req.MangaID).Scan(&progress.CurrentChapter, &progress.Status, &progress.UpdatedAt)
	if err != nil {
		dbErr := NewDatabaseNotFoundError()
		log.Info("progress_not_found", "manga_id", req.MangaID)
		SendError(client, dbErr)
		return dbErr
	}

	log.Debug("progress_retrieved", "manga_id", req.MangaID)
	client.Conn.Write(CreateDataMessage("progress", progress))
	return nil
}

func handleAddToLibrary(client *Client, payload json.RawMessage, log *logger.Logger, br *bridge.Bridge) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	log = log.WithFields(map[string]interface{}{
		"user_id":  client.UserID,
		"username": client.Username,
	})

	var req AddToLibraryPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid add_to_library payload")
		SendError(client, protoErr)
		return protoErr
	}

	if req.MangaID == "" {
		bizErr := NewBizInvalidMangaIDError()
		SendError(client, bizErr)
		return bizErr
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
		bizErr := NewBizInvalidStatusError(status)
		SendError(client, bizErr)
		return bizErr
	}

	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)`
	err := database.DB.QueryRow(checkQuery, req.MangaID).Scan(&exists)
	if err != nil || !exists {
		bizErr := NewBizMangaNotFoundError(req.MangaID)
		SendError(client, bizErr)
		return bizErr
	}

	now := time.Now()
	query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
              VALUES (?, ?, 0, ?, ?)
              ON CONFLICT(user_id, manga_id) DO UPDATE SET status = ?, updated_at = ?`

	_, err = database.DB.Exec(query, client.UserID, req.MangaID, status, now, status, now)
	if err != nil {
		dbErr := NewDatabaseQueryError(err)
		log.Error("database_error_adding_to_library", "error", err.Error(), "manga_id", req.MangaID)
		SendError(client, dbErr)
		return dbErr
	}

	log.Info("manga_added_to_library", "manga_id", req.MangaID, "status", status)

	if br != nil {
		br.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{
			UserID:  client.UserID,
			MangaID: req.MangaID,
			Action:  "added",
		})
	}

	client.Conn.Write(CreateSuccessMessage("Manga added to library successfully"))
	return nil
}

func handleRemoveFromLibrary(client *Client, payload json.RawMessage, log *logger.Logger, br *bridge.Bridge) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	log = log.WithFields(map[string]interface{}{
		"user_id":  client.UserID,
		"username": client.Username,
	})

	var req RemoveFromLibraryPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid remove_from_library payload")
		SendError(client, protoErr)
		return protoErr
	}

	if req.MangaID == "" {
		bizErr := NewBizInvalidMangaIDError()
		SendError(client, bizErr)
		return bizErr
	}

	query := `DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?`
	result, err := database.DB.Exec(query, client.UserID, req.MangaID)
	if err != nil {
		dbErr := NewDatabaseQueryError(err)
		log.Error("database_error_removing_from_library", "error", err.Error(), "manga_id", req.MangaID)
		SendError(client, dbErr)
		return dbErr
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		bizErr := NewBizNotInLibraryError(req.MangaID)
		SendError(client, bizErr)
		return bizErr
	}

	log.Info("manga_removed_from_library", "manga_id", req.MangaID)

	if br != nil {
		br.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{
			UserID:  client.UserID,
			MangaID: req.MangaID,
			Action:  "removed",
		})
	}

	client.Conn.Write(CreateSuccessMessage("Manga removed from library successfully"))
	return nil
}

func handleConnect(client *Client, payload json.RawMessage, log *logger.Logger, sessionMgr *SessionManager, heartbeatMgr *HeartbeatManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	var connectPayload ConnectPayload
	if err := json.Unmarshal(payload, &connectPayload); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid connect payload")
		SendError(client, protoErr)
		return protoErr
	}

	session := sessionMgr.CreateSession(client.ID, client.UserID, connectPayload.DeviceType, connectPayload.DeviceName)
	heartbeatMgr.RecordHeartbeat(client.ID, 0)

	log.Info("client_connected_sync",
		"session_id", session.SessionID,
		"device_type", connectPayload.DeviceType,
		"device_name", connectPayload.DeviceName)

	response := CreateConnectResponseMessage(session.SessionID, connectPayload.DeviceType)
	_, err := client.Conn.Write(response)
	if err != nil {
		return NewNetworkWriteError(err)
	}
	return nil
}

func handleDisconnect(client *Client, payload json.RawMessage, log *logger.Logger, sessionMgr *SessionManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	var disconnectPayload DisconnectPayload
	if err := json.Unmarshal(payload, &disconnectPayload); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid disconnect payload")
		SendError(client, protoErr)
		return protoErr
	}

	session, ok := sessionMgr.GetSessionByClientID(client.ID)
	if ok {
		sessionMgr.RemoveSessionByClientID(client.ID)
		log.Info("client_disconnected_sync",
			"session_id", session.SessionID,
			"reason", disconnectPayload.Reason)
	}

	client.Conn.Write(CreateSuccessMessage("Disconnected successfully"))
	return nil
}

func handleHeartbeat(client *Client, payload json.RawMessage, log *logger.Logger, heartbeatMgr *HeartbeatManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	heartbeatMgr.RecordHeartbeat(client.ID, 0)

	log.Debug("heartbeat_received", "client_id", client.ID)

	response := CreateHeartbeatMessage()
	_, err := client.Conn.Write(response)
	if err != nil {
		return NewNetworkWriteError(err)
	}
	return nil
}

func handleStatusRequest(client *Client, log *logger.Logger, sessionMgr *SessionManager, heartbeatMgr *HeartbeatManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	session, ok := sessionMgr.GetSessionByClientID(client.ID)
	if !ok {
		bizErr := NewProtocolInvalidPayloadError("No active session")
		SendError(client, bizErr)
		return bizErr
	}

	lastHeartbeat, ok := heartbeatMgr.GetLastHeartbeat(client.ID)
	var lastHeartbeatStr string
	if ok {
		lastHeartbeatStr = lastHeartbeat.Format(time.RFC3339)
	}

	rtt, _ := heartbeatMgr.GetRTT(client.ID)
	quality := heartbeatMgr.GetNetworkQuality(client.ID)

	uptime := int64(time.Since(session.ConnectedAt).Seconds())

	deviceCount := sessionMgr.GetUserDeviceCount(client.UserID)

	var lastSyncInfo *LastSyncInfo
	if !session.LastSyncTime.IsZero() && session.LastSyncManga != "" {
		lastSyncInfo = &LastSyncInfo{
			MangaID:    session.LastSyncManga,
			MangaTitle: session.LastSyncMangaTitle,
			Chapter:    session.LastSyncChapter,
			Timestamp:  session.LastSyncTime.Format(time.RFC3339),
		}
	}

	statusPayload := StatusResponsePayload{
		ConnectionStatus: "active",
		ServerAddress:    client.Conn.LocalAddr().String(),
		Uptime:           uptime,
		LastHeartbeat:    lastHeartbeatStr,
		SessionID:        session.SessionID,
		DevicesOnline:    deviceCount,
		MessagesSent:     session.MessagesSent,
		MessagesReceived: session.MessagesReceived,
		LastSync:         lastSyncInfo,
		NetworkQuality:   quality,
		RTT:              int64(rtt.Milliseconds()),
	}

	response := CreateStatusResponseMessage(statusPayload)

	log.Debug("status_request_handled",
		"session_id", session.SessionID,
		"uptime", uptime,
		"network_quality", quality)

	_, err := client.Conn.Write(response)
	if err != nil {
		return NewNetworkWriteError(err)
	}
	return nil
}

func handleSubscribeUpdates(client *Client, payload json.RawMessage, log *logger.Logger, sessionMgr *SessionManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	var subscribePayload SubscribeUpdatesPayload
	if err := json.Unmarshal(payload, &subscribePayload); err != nil {
		protoErr := NewProtocolInvalidPayloadError("Invalid subscribe payload")
		SendError(client, protoErr)
		return protoErr
	}

	eventTypes := subscribePayload.EventTypes
	if len(eventTypes) == 0 {
		eventTypes = []string{"progress", "library"}
	}

	if !sessionMgr.Subscribe(client.ID, eventTypes) {
		bizErr := NewProtocolInvalidPayloadError("Failed to subscribe")
		SendError(client, bizErr)
		return bizErr
	}

	log.Info("client_subscribed",
		"user_id", client.UserID,
		"event_types", eventTypes)

	client.Conn.Write(CreateSuccessMessage("Subscribed to updates"))
	return nil
}

func handleUnsubscribeUpdates(client *Client, log *logger.Logger, sessionMgr *SessionManager) error {
	if !client.Authenticated {
		authErr := NewAuthNotAuthenticatedError()
		SendError(client, authErr)
		return authErr
	}

	if !sessionMgr.Unsubscribe(client.ID) {
		bizErr := NewProtocolInvalidPayloadError("Failed to unsubscribe")
		SendError(client, bizErr)
		return bizErr
	}

	log.Info("client_unsubscribed", "user_id", client.UserID)

	client.Conn.Write(CreateSuccessMessage("Unsubscribed from updates"))
	return nil
}
