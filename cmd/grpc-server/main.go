package main

import (
	"log"
	"net"
	"os"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/grpc"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	pb "github.com/binhbb2204/Manga-Hub-Group13/proto/manga"
	"github.com/joho/godotenv"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using default values")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/mangahub.db"
	}
	if err := database.InitDatabase(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = ":50051"
	}
	if port[0] != ':' {
		port = ":" + port
	}

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := googlegrpc.NewServer()
	mangaServer := grpc.NewServer(database.DB)
	pb.RegisterMangaServiceServer(s, mangaServer)
	reflection.Register(s)

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
