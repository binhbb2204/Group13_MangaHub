package bridge

import "time"

type EventType string

const (
	EventTypeProgressUpdate EventType = "progress_update"
	EventTypeLibraryUpdate  EventType = "library_update"
	EventTypeUserMessage    EventType = "user_message"
)

type Event struct {
	Type      EventType              `json:"type"`
	UserID    string                 `json:"user_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

type ProgressUpdateEvent struct {
	UserID       string    `json:"user_id"`
	MangaID      string    `json:"manga_id"`
	ChapterID    int       `json:"chapter_id"`
	Status       string    `json:"status"`
	LastReadDate time.Time `json:"last_read_date"`
}

type LibraryUpdateEvent struct {
	UserID  string `json:"user_id"`
	MangaID string `json:"manga_id"`
	Action  string `json:"action"`
}
