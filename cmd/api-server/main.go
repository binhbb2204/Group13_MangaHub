package main

import (
	"log"
	"os"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/auth"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env if present (optional)
	_ = godotenv.Load()

	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	if err := database.InitDatabase(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Get JWT secret from environment or use default (change in production!)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
		log.Println("Warning: Using default JWT secret. Set JWT_SECRET environment variable in production!")
	}

	//frontend URL from environment or use default
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
		log.Println("Using default frontend URL: http://localhost:3000")
	}

	// Initialize handlers
	authHandler := auth.NewHandler(jwtSecret)
	mangaHandler := manga.NewHandler()
	userHandler := user.NewHandler()

	// Setup Gin router
	router := gin.Default()

	// CORS middleware configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{frontendURL}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.ExposeHeaders = []string{"Content-Length"}
	config.AllowCredentials = true
	router.Use(cors.New(config))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth routes (public)
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
	}
	// Protected account routes
	protectedAuth := router.Group("/auth")
	protectedAuth.Use(auth.AuthMiddleware(jwtSecret))
	{
		protectedAuth.POST("/change-password", authHandler.ChangePassword)
	}

	// Manga routes (public for search, protected for create)
	mangaGroup := router.Group("/manga")
	{
		mangaGroup.GET("", mangaHandler.SearchManga)      // Search manga
		mangaGroup.GET("/all", mangaHandler.GetAllManga)  // Get all manga
		mangaGroup.GET("/:id", mangaHandler.GetMangaByID) // Get manga by ID

		// Protected routes
		protected := mangaGroup.Group("")
		protected.Use(auth.AuthMiddleware(jwtSecret))
		{
			protected.POST("", mangaHandler.CreateManga) // Create manga (for testing)
		}
	}

	// User routes (all protected)
	userGroup := router.Group("/users")
	userGroup.Use(auth.AuthMiddleware(jwtSecret))
	{
		userGroup.GET("/me", userHandler.GetProfile)                          // Get current user profile
		userGroup.POST("/library", userHandler.AddToLibrary)                  // Add manga to library
		userGroup.GET("/library", userHandler.GetLibrary)                     // Get user's library
		userGroup.PUT("/progress", userHandler.UpdateProgress)                // Update reading progress
		userGroup.DELETE("/library/:manga_id", userHandler.RemoveFromLibrary) // Remove from library
	}

	// Get port from environment or use default
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting API server on port %s...\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
