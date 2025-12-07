package grpc

import (
	"context"
	"database/sql"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	pb "github.com/binhbb2204/Manga-Hub-Group13/proto/manga"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedMangaServiceServer
	DB          *sql.DB
	broadcaster *GRPCBroadcaster
	bridge      *bridge.UnifiedBridge
}

func NewServer(db *sql.DB) *Server {
	broadcaster := NewGRPCBroadcaster(logger.GetLogger())
	return &Server{
		DB:          db,
		broadcaster: broadcaster,
		bridge:      nil,
	}
}

func (s *Server) SetBridge(b *bridge.UnifiedBridge) {
	s.bridge = b
	s.broadcaster.SetBridge(b)
}

func (s *Server) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "manga ID is required")
	}

	var manga pb.MangaResponse
	var genres string

	query := `SELECT id, title, author, genres, status, total_chapters, description FROM manga WHERE id = ?`
	err := s.DB.QueryRowContext(ctx, query, req.Id).Scan(
		&manga.Id, &manga.Title, &manga.Author, &genres, &manga.Status, &manga.TotalChapters, &manga.Description,
	)

	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "manga not found: %s", req.Id)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query manga: %v", err)
	}

	manga.Genres = parseGenres(genres)
	return &manga, nil
}

func (s *Server) SearchManga(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString("SELECT id, title, author, genres, status, total_chapters, description FROM manga WHERE 1=1")
	var args []interface{}

	if req.Query != "" {
		queryBuilder.WriteString(" AND (title LIKE ? OR author LIKE ?)")
		searchTerm := "%" + req.Query + "%"
		args = append(args, searchTerm, searchTerm)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	queryBuilder.WriteString(" LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := s.DB.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search manga: %v", err)
	}
	defer rows.Close()

	var mangas []*pb.MangaResponse
	for rows.Next() {
		var m pb.MangaResponse
		var genres string
		if err := rows.Scan(&m.Id, &m.Title, &m.Author, &genres, &m.Status, &m.TotalChapters, &m.Description); err != nil {
			continue
		}
		m.Genres = parseGenres(genres)
		mangas = append(mangas, &m)
	}

	return &pb.SearchResponse{
		Mangas:     mangas,
		TotalCount: int32(len(mangas)),
	}, nil
}

func (s *Server) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	if req.UserId == "" || req.MangaId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and manga_id are required")
	}

	query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, updated_at) 
              VALUES (?, ?, ?, CURRENT_TIMESTAMP) 
              ON CONFLICT(user_id, manga_id) DO UPDATE SET 
              current_chapter = excluded.current_chapter, 
              updated_at = CURRENT_TIMESTAMP`

	_, err := s.DB.ExecContext(ctx, query, req.UserId, req.MangaId, req.Chapter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update progress: %v", err)
	}

	if s.bridge != nil {
		event := bridge.NewUnifiedEvent(
			bridge.EventProgressUpdate,
			req.UserId,
			bridge.ProtocolGRPC,
			map[string]interface{}{
				"manga_id": req.MangaId,
				"chapter":  req.Chapter,
			},
		)
		s.bridge.BroadcastEvent(event)
	}

	return &pb.ProgressResponse{
		Success: true,
		Message: "Progress updated successfully",
	}, nil
}

func parseGenres(genresStr string) []string {
	cleaned := strings.ReplaceAll(genresStr, "[", "")
	cleaned = strings.ReplaceAll(cleaned, "]", "")
	cleaned = strings.ReplaceAll(cleaned, "\"", "")
	if cleaned == "" {
		return []string{}
	}
	return strings.Split(cleaned, ",")
}
