package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/auth"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/grpc"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/health"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/metrics"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	grpc_server "google.golang.org/grpc"
)

type ServerOrchestrator struct {
	logger       *logger.Logger
	bridge       *bridge.UnifiedBridge
	oldBridge    *bridge.Bridge
	tcpServer    *tcp.Server
	udpServer    *udp.Server
	wsServer     *websocket.Server
	grpcServer   *grpc.Server
	grpcListener *grpc_server.Server
	httpRouter   *gin.Engine
	db           *sql.DB
	config       *Config
	stopChan     chan os.Signal
}

type Config struct {
	APIPort       string
	TCPPort       string
	UDPPort       string
	WebSocketPort string
	GRPCPort      string
	JWTSecret     string
	FrontendURL   string
	LocalIP       string
	EnableAPI     bool
	EnableTCP     bool
	EnableUDP     bool
	EnableWS      bool
	EnableGRPC    bool
}

func NewServerOrchestrator(db *sql.DB, cfg *Config) *ServerOrchestrator {
	log := logger.GetLogger()

	unifiedBridge := bridge.NewUnifiedBridge(log)

	return &ServerOrchestrator{
		logger:   log,
		bridge:   unifiedBridge,
		db:       db,
		config:   cfg,
		stopChan: make(chan os.Signal, 1),
	}
}

func (o *ServerOrchestrator) initializeServers() error {
	o.logger.Info("initializing_servers")

	// Initialize HTTP API Server
	if o.config.EnableAPI {
		o.logger.Info("initializing_api_server", "port", o.config.APIPort)

		// Create old bridge wrapper for compatibility with existing user handler
		o.oldBridge = bridge.NewBridge(o.logger)
		o.oldBridge.Start()

		// Initialize handlers
		authHandler := auth.NewHandler(o.config.JWTSecret)
		mangaHandler := manga.NewHandler()
		userHandler := user.NewHandler(o.oldBridge)
		healthHandler := health.NewHandler(o.oldBridge)
		metricsHandler := metrics.NewHandler()

		gin.SetMode(gin.ReleaseMode)
		router := gin.Default()

		// CORS configuration
		config := cors.DefaultConfig()
		config.AllowOrigins = []string{o.config.FrontendURL}
		config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
		config.ExposeHeaders = []string{"Content-Length"}
		config.AllowCredentials = true
		router.Use(cors.New(config))

		// Health routes
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok", "local_ip": o.config.LocalIP})
		})
		router.GET("/healthz", healthHandler.Healthz)
		router.GET("/readyz", healthHandler.Readyz)
		router.GET("/metrics", metricsHandler.Metrics)

		// Auth routes
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
			auth.AuthMiddleware(o.config.JWTSecret)(c)
		})
		{
			protectedAuth.POST("/change-password", authHandler.ChangePassword)
			protectedAuth.POST("/update-email", authHandler.UpdateEmail)
			protectedAuth.POST("/update-username", authHandler.UpdateUsername)
		}

		// Manga routes
		mangaGroup := router.Group("/manga")
		{
			mangaGroup.GET("", mangaHandler.SearchManga)
			mangaGroup.GET("/all", mangaHandler.GetAllManga)
			mangaGroup.GET("/search", mangaHandler.SearchManga)
			mangaGroup.GET("/search-external", mangaHandler.SearchExternal)
			mangaGroup.GET("/info/:id", mangaHandler.GetMangaInfo)
			mangaGroup.GET("/featured", mangaHandler.GetFeaturedManga)
			mangaGroup.GET("/chapters/:mangadexId", mangaHandler.GetChapters)
			mangaGroup.GET("/chapter/:chapterId/pages", mangaHandler.GetChapterPages)
			mangaGroup.GET("/:id", mangaHandler.GetMangaByID)

			protected := mangaGroup.Group("")
			protected.Use(auth.AuthMiddleware(o.config.JWTSecret))
			{
				protected.POST("", mangaHandler.CreateManga)
			}
		}

		// User routes (all protected)
		userGroup := router.Group("/users")
		userGroup.Use(func(c *gin.Context) {
			c.Set("authHandler", authHandler)
			auth.AuthMiddleware(o.config.JWTSecret)(c)
		})
		{
			userGroup.GET("/me", userHandler.GetProfile)
			userGroup.POST("/library", userHandler.AddToLibrary)
			userGroup.GET("/library", userHandler.GetLibrary)
			userGroup.GET("/progress/:manga_id", userHandler.GetProgress)
			userGroup.PUT("/progress", userHandler.UpdateProgress)
			userGroup.DELETE("/library/:manga_id", userHandler.RemoveFromLibrary)
		}

		o.httpRouter = router
	}

	if o.config.EnableTCP {
		o.logger.Info("initializing_tcp_server", "port", o.config.TCPPort)

		// Ensure a bridge exists so HTTP can signal TCP and vice-versa
		// If API didn't initialize oldBridge, create and start it here.
		if o.oldBridge == nil {
			o.oldBridge = bridge.NewBridge(o.logger)
			o.oldBridge.Start()
		}

		// Wire TCP server to the same bridge used by HTTP handlers
		o.tcpServer = tcp.NewServer(o.config.TCPPort, o.oldBridge)
	}

	if o.config.EnableUDP {
		o.logger.Info("initializing_udp_server", "port", o.config.UDPPort)
		o.udpServer = udp.NewServer(o.config.UDPPort)
		o.udpServer.SetBridge(o.bridge)
	}

	if o.config.EnableWS {
		o.logger.Info("initializing_websocket_server", "port", o.config.WebSocketPort)
		o.wsServer = websocket.NewServer(o.db, o.config.JWTSecret)
		o.wsServer.SetBridge(o.bridge)
	}

	if o.config.EnableGRPC {
		o.logger.Info("initializing_grpc_server", "port", o.config.GRPCPort)
		o.grpcServer = grpc.NewServer(o.db)
		o.grpcServer.SetBridge(o.bridge)
	}

	o.logger.Info("servers_initialized")
	return nil
}

func (o *ServerOrchestrator) Start() error {
	if err := o.initializeServers(); err != nil {
		return fmt.Errorf("failed to initialize servers: %w", err)
	}

	o.bridge.Start()
	o.logger.Info("unified_bridge_started")

	errChan := make(chan error, 5)

	// Start HTTP API Server
	if o.config.EnableAPI && o.httpRouter != nil {
		go func() {
			o.logger.Info("starting_http_api_server", "bind", "0.0.0.0:"+o.config.APIPort, "local_ip", o.config.LocalIP)
			if err := o.httpRouter.Run("0.0.0.0:" + o.config.APIPort); err != nil {
				o.logger.Error("http_api_server_start_failed", "error", err.Error())
				errChan <- fmt.Errorf("HTTP API server: %w", err)
			}
		}()
	}

	if o.config.EnableTCP && o.tcpServer != nil {
		go func() {
			o.logger.Info("starting_tcp_server", "port", o.config.TCPPort, "local_ip", o.config.LocalIP)
			if err := o.tcpServer.Start(); err != nil {
				o.logger.Error("tcp_server_start_failed", "error", err.Error())
				errChan <- fmt.Errorf("TCP server: %w", err)
			}
		}()
	}

	if o.config.EnableUDP && o.udpServer != nil {
		go func() {
			o.logger.Info("starting_udp_server", "port", o.config.UDPPort, "local_ip", o.config.LocalIP)
			if err := o.udpServer.Start(); err != nil {
				o.logger.Error("udp_server_start_failed", "error", err.Error())
				errChan <- fmt.Errorf("UDP server: %w", err)
			}
		}()

		// Wait a bit for UDP server to initialize, then connect oldBridge to UDP broadcaster
		go func() {
			time.Sleep(200 * time.Millisecond)
			// Get the UDP broadcaster from the unified bridge and set it on the old bridge
			if o.oldBridge != nil && o.bridge != nil {
				udpBroadcaster := o.bridge.GetUDPBroadcaster()
				if udpBroadcaster != nil {
					// The UDP Broadcaster implementation (*udp.Broadcaster) implements both interfaces
					// Check if it implements OldUDPBroadcaster and use it
					if oldStyleBroadcaster, ok := udpBroadcaster.(bridge.OldUDPBroadcaster); ok {
						o.oldBridge.SetUDPBroadcaster(oldStyleBroadcaster)
						o.logger.Info("old_bridge_connected_to_udp")
					} else {
						o.logger.Warn("udp_broadcaster_incompatible_with_old_bridge")
					}
				}
			}
		}()
	}

	if o.config.EnableWS && o.wsServer != nil {
		go func() {
			o.logger.Info("ws_server_ready", "port", o.config.WebSocketPort, "local_ip", o.config.LocalIP)
		}()
	}

	if o.config.EnableGRPC && o.grpcServer != nil {
		go func() {
			o.logger.Info("grpc_server_ready", "port", o.config.GRPCPort, "local_ip", o.config.LocalIP)
		}()
	}

	select {
	case err := <-errChan:
		o.logger.Error("server_start_error", "error", err.Error())
		return err
	case <-time.After(500 * time.Millisecond):
		o.logger.Info("all_servers_started_successfully")
	}

	signal.Notify(o.stopChan, os.Interrupt, syscall.SIGTERM)

	return nil
}

func (o *ServerOrchestrator) WaitForShutdown() {
	sig := <-o.stopChan
	o.logger.Info("shutdown_signal_received", "signal", sig.String())
	o.Shutdown()
}

func (o *ServerOrchestrator) Shutdown() {
	o.logger.Info("orchestrator_shutting_down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		if o.tcpServer != nil {
			o.logger.Info("stopping_tcp_server")
			o.tcpServer.Stop()
		}

		if o.udpServer != nil {
			o.logger.Info("stopping_udp_server")
			o.udpServer.Stop()
		}

		if o.grpcListener != nil {
			o.logger.Info("stopping_grpc_server")
			o.grpcListener.GracefulStop()
		}

		if o.bridge != nil {
			o.logger.Info("stopping_unified_bridge")
			o.bridge.Stop()
		}

		close(done)
	}()

	select {
	case <-done:
		o.logger.Info("graceful_shutdown_complete")
	case <-ctx.Done():
		o.logger.Warn("shutdown_timeout_forcing_stop")
	}
}

func main() {
	logger.Init(logger.INFO, false, os.Stdout)
	log := logger.GetLogger()

	log.Info("manga_hub_orchestrator_starting")

	// Detect local IP early
	localIP := utils.GetLocalIP()
	log.Info("local_ip_detected", "ip", localIP)

	dbPath := getEnv("DB_PATH", "./data/mangahub.db")
	if err := database.InitDatabase(dbPath); err != nil {
		log.Error("database_init_failed", "error", err.Error())
		os.Exit(1)
	}
	defer database.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
		log.Warn("using_default_jwt_secret")
	}

	config := &Config{
		APIPort:       getEnv("API_PORT", "8080"),
		TCPPort:       getEnv("TCP_PORT", "9090"),
		UDPPort:       getEnv("UDP_PORT", "9091"),
		WebSocketPort: getEnv("WS_PORT", "9093"),
		GRPCPort:      getEnv("GRPC_PORT", "9092"),
		JWTSecret:     jwtSecret,
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:3000"),
		LocalIP:       localIP,
		EnableAPI:     getEnvBool("ENABLE_API", true),
		EnableTCP:     getEnvBool("ENABLE_TCP", true),
		EnableUDP:     getEnvBool("ENABLE_UDP", true),
		EnableWS:      getEnvBool("ENABLE_WS", true),
		EnableGRPC:    getEnvBool("ENABLE_GRPC", true),
	}

	orchestrator := NewServerOrchestrator(database.DB, config)

	if err := orchestrator.Start(); err != nil {
		log.Error("orchestrator_start_failed", "error", err.Error())
		os.Exit(1)
	}

	log.Info("manga_hub_running",
		"api", config.EnableAPI,
		"tcp", config.EnableTCP,
		"udp", config.EnableUDP,
		"websocket", config.EnableWS,
		"grpc", config.EnableGRPC)

	orchestrator.WaitForShutdown()
	log.Info("manga_hub_stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
