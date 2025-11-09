package models

import "time"

type UserProgress struct {
	UserID         string    `json:"user_id" db:"user_id"`
	MangaID        string    `json:"manga_id" db:"manga_id"`
	CurrentChapter int       `json:"current_chapter" db:"current_chapter"`
	Status         string    `json:"status" db:"status"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type AddToLibraryRequest struct {
	MangaID string `json:"manga_id" binding:"required"`
	Status  string `json:"status" binding:"required,oneof=reading completed plan_to_read"`
}

type UpdateProgressRequest struct {
	MangaID        string `json:"manga_id" binding:"required"`
	CurrentChapter int    `json:"current_chapter" binding:"required,min=0"`
	Status         string `json:"status" binding:"omitempty,oneof=reading completed plan_to_read"`
}

type MangaProgress struct {
	Manga          Manga     `json:"manga"`
	CurrentChapter int       `json:"current_chapter"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserLibrary struct {
	Reading    []MangaProgress `json:"reading"`
	Completed  []MangaProgress `json:"completed"`
	PlanToRead []MangaProgress `json:"plan_to_read"`
}
