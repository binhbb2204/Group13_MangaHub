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
}

type UpdateProgressRequest struct {
	MangaID        string   `json:"manga_id" binding:"required"`
	CurrentChapter *int     `json:"current_chapter" binding:"omitempty,min=0"` // Optional
	Status         string   `json:"status" binding:"omitempty,oneof=reading completed plan_to_read"`
	UserRating     *float64 `json:"user_rating" binding:"omitempty,min=1,max=5"` // User's rating 1-5 stars
}

type MangaProgress struct {
	Manga          Manga     `json:"manga"`
	CurrentChapter int       `json:"current_chapter"`
	Status         string    `json:"status"`
	UserRating     *float64  `json:"user_rating"` // Pointer so null is explicit
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserLibrary struct {
	Reading    []MangaProgress `json:"reading"`
	Completed  []MangaProgress `json:"completed"`
	PlanToRead []MangaProgress `json:"plan_to_read"`
}
