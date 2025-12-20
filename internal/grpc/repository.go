package grpc

import (
	"context"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
)

type SearchFilter struct {
	Query  string
	Author string
	Status string
	Genres []string

	Limit  int32
	Offset int32
}

type MangaRepository interface {
	AddManga(ctx context.Context, manga *models.Manga) (*models.Manga, error)

	GetMangaByID(ctx context.Context, id string) (*models.Manga, error)

	SearchManga(
		ctx context.Context,
		filter *SearchFilter,
	) ([]*models.Manga, int32, error)

	UpdateMangaProgress(
		ctx context.Context,
		userID string,
		mangaID string,
		chapter int32,
	) error
}
