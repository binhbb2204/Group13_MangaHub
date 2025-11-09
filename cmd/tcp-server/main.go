package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	logLevel := logger.INFO
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		logLevel = logger.LogLevel(level)
	}
	jsonFormat := os.Getenv("LOG_FORMAT") == "json"
	logger.Init(logLevel, jsonFormat, os.Stdout)

	log := logger.GetLogger().WithContext("component", "main")
	log.Info("starting_tcp_server", "version", "1.0.0")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	if err := database.InitDatabase(dbPath); err != nil {
		log.Error("failed_to_initialize_database", "error", err.Error(), "path", dbPath)
		os.Exit(1)
	}
	defer database.Close()

	port := os.Getenv("TCP_PORT")
	if port == "" {
		port = "9090"
	}

	tcpBridge := bridge.NewBridge(logger.WithContext("component", "bridge"))
	tcpBridge.Start()
	defer tcpBridge.Stop()

	server := tcp.NewServer(port, tcpBridge)
	if err := server.Start(); err != nil {
		log.Error("failed_to_start_tcp_server", "error", err.Error())
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Info("tcp_server_ready", "port", port)
	<-sigChan

	log.Info("shutdown_signal_received")
	if err := server.Stop(); err != nil {
		log.Error("error_stopping_server", "error", err.Error())
	}

	log.Info("tcp_server_shutdown_complete")
}
