package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/auth"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/health"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/metrics"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

	log := logger.GetLogger().WithContext("component", "api_server")
	log.Info("starting_api_server", "version", "1.0.0")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	if err := database.InitDatabase(dbPath); err != nil {
		log.Error("failed_to_initialize_database", "error", err.Error(), "path", dbPath)
		os.Exit(1)
	}
	defer database.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
		log.Warn("using_default_jwt_secret", "message", "Set JWT_SECRET environment variable in production!")
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	localIP := utils.GetLocalIP()
	if frontendURL == "" {
		// Auto-detect frontend URL - try local IP first, then fallback to localhost
		if localIP != "" && localIP != "127.0.0.1" {
			frontendURL = fmt.Sprintf("http://%s:3000", localIP)
		} else {
			frontendURL = "http://localhost:3000"
		}
		log.Info("using_frontend_url", "url", frontendURL)
	}

	apiBridge := bridge.NewBridge(logger.GetLogger())
	apiBridge.Start()
	defer apiBridge.Stop()
	log.Info("tcp_http_bridge_started")

	authHandler := auth.NewHandler(jwtSecret)
	// Inject authHandler into Gin context for middleware
	gin.SetMode(gin.ReleaseMode)
	mangaHandler := manga.NewHandler()
	userHandler := user.NewHandler(apiBridge)
	healthHandler := health.NewHandler(apiBridge)
	metricsHandler := metrics.NewHandler()

	router := gin.Default()

	config := cors.DefaultConfig()

	// Build allowed origins list: env FRONTEND_URL, localhost variants, detected local IP, and comma-separated FRONTEND_ORIGINS
	allowedOrigins := []string{frontendURL, "http://localhost:3000", "http://127.0.0.1:3000"}
	if localIP != "" && localIP != "127.0.0.1" {
		allowedOrigins = append(allowedOrigins, fmt.Sprintf("http://%s:3000", localIP))
	}
	if extra := os.Getenv("FRONTEND_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	config.AllowOriginFunc = func(origin string) bool {
		// Exact match against allowed origins
		for _, o := range allowedOrigins {
			if origin == o {
				return true
			}
		}
		// Allow LAN hosts on port 3000/5173 common dev ports
		if strings.HasPrefix(origin, "http://192.168.") || strings.HasPrefix(origin, "http://10.") || strings.HasPrefix(origin, "http://172.") {
			return true
		}
		return false
	}

	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "Cache-Control"}
	config.ExposeHeaders = []string{"Content-Length", "Content-Type"}
	config.AllowCredentials = true
	router.Use(cors.New(config))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "local_ip": os.Getenv("LOCAL_IP")})
	})
	router.GET("/healthz", healthHandler.Healthz)
	router.GET("/readyz", healthHandler.Readyz)
	router.GET("/metrics", metricsHandler.Metrics)

	// SSE notifications endpoint
	router.GET("/events", mangaHandler.GetBroker().ServeSSE)

	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", func(c *gin.Context) {
			c.Set("authHandler", authHandler)
			authHandler.Logout(c)
		})
	}

	protectedAuth := router.Group("/auth")
	protectedAuth.Use(func(c *gin.Context) {
		c.Set("authHandler", authHandler)
		auth.AuthMiddleware(jwtSecret)(c)
	})
	{
		protectedAuth.POST("/change-password", authHandler.ChangePassword)
		protectedAuth.POST("/update-email", authHandler.UpdateEmail)
		protectedAuth.POST("/update-username", authHandler.UpdateUsername)
	}

	mangaGroup := router.Group("/manga")
	{
		mangaGroup.GET("", mangaHandler.SearchManga)
		mangaGroup.GET("/all", mangaHandler.GetAllManga)
		mangaGroup.GET("/search", mangaHandler.SearchManga)             // Use DB search
		mangaGroup.GET("/search-external", mangaHandler.SearchExternal) // MAL API fallback
		mangaGroup.GET("/info/:id", mangaHandler.GetMangaInfo)
		mangaGroup.GET("/featured", mangaHandler.GetFeaturedManga)
		// MangaDex chapter routes (must be before /:id to avoid conflicts)
		mangaGroup.GET("/chapters/:mangadexId", mangaHandler.GetChapters)
		mangaGroup.GET("/chapter/:chapterId/pages", mangaHandler.GetChapterPages)
		// Generic ID route must be last
		mangaGroup.GET("/:id", mangaHandler.GetMangaByID)
		// Protected routes
		protected := mangaGroup.Group("")
		protected.Use(auth.AuthMiddleware(jwtSecret))
		{
			protected.POST("", mangaHandler.CreateManga)
			protected.POST("/:id/refresh", mangaHandler.RefreshManga)
		}

		// Admin-only routes
		admin := mangaGroup.Group("")
		admin.Use(auth.AuthMiddleware(jwtSecret))
		admin.Use(auth.AdminMiddleware())
		{
			admin.POST("/refresh-all", mangaHandler.RefreshAllManga)
		}
	}

	// User routes (all protected)
	userGroup := router.Group("/users")
	// userGroup.Use(auth.AuthMiddleware(jwtSecret))
	userGroup.Use(func(c *gin.Context) {
		c.Set("authHandler", authHandler)
		auth.AuthMiddleware(jwtSecret)(c)
	})
	{
		userGroup.GET("/me", userHandler.GetProfile)                          // Get current user profile
		userGroup.POST("/library", userHandler.AddToLibrary)                  // Add manga to library
		userGroup.GET("/library", userHandler.GetLibrary)                     // Get user's library
		userGroup.GET("/progress/:manga_id", userHandler.GetProgress)         // Get progress for specific manga
		userGroup.PUT("/progress", userHandler.UpdateProgress)                // Update reading progress
		userGroup.DELETE("/library/:manga_id", userHandler.RemoveFromLibrary) // Remove from library
	}

	// Debug routes (protected)
	debugGroup := router.Group("/debug")
	debugGroup.Use(func(c *gin.Context) {
		c.Set("authHandler", authHandler)
		auth.AuthMiddleware(jwtSecret)(c)
	})
	{
		// Manually trigger TCP forward when running servers separately
		debugGroup.POST("/forward-test", userHandler.ForwardProgressTest)
	}

	// Sync routes (protected)
	syncGroup := router.Group("/sync")
	syncGroup.Use(func(c *gin.Context) {
		c.Set("authHandler", authHandler)
		auth.AuthMiddleware(jwtSecret)(c)
	})
	{
		syncGroup.POST("/connect", userHandler.SyncConnect)       // Connect to sync server
		syncGroup.GET("/status", userHandler.SyncGetStatus)       // Get sync status
		syncGroup.POST("/disconnect", userHandler.SyncDisconnect) // Disconnect from sync server
	}

	//Get port from environment or use default
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	localIP = utils.GetLocalIP()
	fmt.Printf("\n🚀 API Server Configuration:\n")
	fmt.Printf("   Bind Address:  0.0.0.0:%s\n", port)
	fmt.Printf("   IPv4 Address:  %s\n", localIP)
	fmt.Printf("   API URL:       http://%s:%s\n", localIP, port)
	fmt.Printf("   Health Check:  http://%s:%s/health\n\n", localIP, port)

	log.Info("api_server_starting", "bind", fmt.Sprintf("0.0.0.0:%s", port), "local_ip", localIP, "url", fmt.Sprintf("http://%s:%s", localIP, port))
	if err := router.Run("0.0.0.0:" + port); err != nil {
		log.Error("failed_to_start_api_server", "error", err.Error())
		os.Exit(1)
	}
}
