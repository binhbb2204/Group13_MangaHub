package models

type RatingStats struct {
	Average      float64        `json:"average"`
	TotalCount   int            `json:"total_count"`
	Distribution map[string]int `json:"distribution"`
}
