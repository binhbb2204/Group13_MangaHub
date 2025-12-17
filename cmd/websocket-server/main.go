package main

import (
	"fmt"
	"log"
	"os"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
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

	// Detect local IP for multi-device connectivity
	localIP := utils.GetLocalIP()
	logger.Info("Local IP detected", map[string]interface{}{"ip": localIP})

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/ws/chat", wsServer.HandleWebSocket)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "local_ip": localIP})
	})

	// Log connection info for discovery
	fmt.Printf("\n🚀 WebSocket Server Configuration:\n")
	fmt.Printf("   Bind Address:  0.0.0.0:%s\n", port)
	fmt.Printf("   IPv4 Address:  %s\n", localIP)
	fmt.Printf("   WebSocket URL: ws://%s:%s/ws/chat\n", localIP, port)
	fmt.Printf("   Health Check:  http://%s:%s/health\n\n", localIP, port)

	logger.Info("WebSocket server starting", map[string]interface{}{
		"bind":     fmt.Sprintf("0.0.0.0:%s", port),
		"local_ip": localIP,
		"url":      fmt.Sprintf("ws://%s:%s/ws/chat", localIP, port),
	})

	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatal(err)
	}
}
