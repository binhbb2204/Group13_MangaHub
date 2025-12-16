package main

import (
	"log"
	"os"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	logLevel := logger.INFO
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		logLevel = logger.DEBUG
	}
	logger.Init(logLevel, false, os.Stdout)

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	if err := database.InitDatabase(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.DB.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET not set")
	}

	wsServer := websocket.NewServer(database.DB, jwtSecret)

	port := os.Getenv("WEBSOCKET_PORT")
	if port == "" {
		port = "9093"
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/ws/chat", wsServer.HandleWebSocket)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	logger.Info("WebSocket server starting", map[string]interface{}{"port": port})
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
