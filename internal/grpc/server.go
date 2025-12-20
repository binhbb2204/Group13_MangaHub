package grpc

import (
	"context"
	"database/sql"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	pb "github.com/binhbb2204/Manga-Hub-Group13/proto/manga"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedMangaServiceServer
	DB          *sql.DB
	repository  MangaRepository
	broadcaster *GRPCBroadcaster
	bridge      *bridge.UnifiedBridge
}

func NewServer(db *sql.DB) *Server {
	broadcaster := NewGRPCBroadcaster(logger.GetLogger())

	var repo MangaRepository
	if db != nil {
		repo = NewDBRepository(db)
	} else {
		repo = NewMemoryRepository()
	}

	return &Server{
		DB:          db,
		repository:  repo,
		broadcaster: broadcaster,
	}
}

func NewServerWithRepository(repo MangaRepository, db *sql.DB) *Server {
	broadcaster := NewGRPCBroadcaster(logger.GetLogger())

	return &Server{
		DB:          db,
		repository:  repo,
		broadcaster: broadcaster,
	}
}

func (s *Server) SetBridge(b *bridge.UnifiedBridge) {
	s.bridge = b
	s.broadcaster.SetBridge(b)
}

func (s *Server) AddManga(ctx context.Context, req *pb.AddMangaRequest) (*pb.AddMangaResponse, error) {
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "manga title is required")
	}

	manga := &models.Manga{
		Title:         req.Title,
		Author:        req.Author,
		Genres:        req.Genres,
		Status:        req.Status,
		TotalChapters: int(req.TotalChapters),
		Description:   req.Description,
		CoverURL:      req.CoverUrl,
		MediaType:     req.MediaType,
	}

	created, err := s.repository.AddManga(ctx, manga)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add manga: %v", err)
	}

	return &pb.AddMangaResponse{
		Id:            created.ID,
		Title:         created.Title,
		Author:        created.Author,
		Genres:        created.Genres,
		Status:        created.Status,
		TotalChapters: int32(created.TotalChapters),
		Description:   created.Description,
		CoverUrl:      created.CoverURL,
		MediaType:     created.MediaType,
	}, nil
}

func (s *Server) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "manga ID is required")
	}

	manga, err := s.repository.GetMangaByID(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "manga not found: %s", req.Id)
	}

	return &pb.MangaResponse{
		Id:            manga.ID,
		Title:         manga.Title,
		Author:        manga.Author,
		Genres:        manga.Genres,
		Status:        manga.Status,
		TotalChapters: int32(manga.TotalChapters),
		Description:   manga.Description,
	}, nil
}

func (s *Server) SearchManga(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	filter := &SearchFilter{
		Query:  req.Query,
		Author: req.Author,
		Genres: req.Genres,
		Status: req.Status,
		Limit:  limit,
		Offset: offset,
	}

	mangas, total, err := s.repository.SearchManga(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search manga: %v", err)
	}

	res := make([]*pb.MangaResponse, 0, len(mangas))
	for _, m := range mangas {
		res = append(res, &pb.MangaResponse{
			Id:            m.ID,
			Title:         m.Title,
			Author:        m.Author,
			Genres:        m.Genres,
			Status:        m.Status,
			TotalChapters: int32(m.TotalChapters),
			Description:   m.Description,
		})
	}

	return &pb.SearchResponse{
		Mangas:     res,
		TotalCount: total,
	}, nil
}

func (s *Server) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	if req.UserId == "" || req.MangaId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and manga_id are required")
	}

	if err := s.repository.UpdateMangaProgress(ctx, req.UserId, req.MangaId, req.Chapter); err != nil {
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
