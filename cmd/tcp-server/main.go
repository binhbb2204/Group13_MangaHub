package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	if err := database.InitDatabase(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	port := os.Getenv("TCP_PORT")
	if port == "" {
		port = "9090"
	}

	server := tcp.NewServer(port)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("TCP server started. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down TCP server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	log.Println("Server stopped gracefully")
}
