package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
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

	log := logger.GetLogger().WithContext("component", "udp_main")
	log.Info("starting_udp_server", "version", "1.0.0")

	port := os.Getenv("UDP_PORT")
	if port == "" {
		port = "9091"
		log.Warn("using_default_port", "port", port)
	}

	httpPort := os.Getenv("UDP_HTTP_PORT")
	if httpPort == "" {
		httpPort = "9092"
		log.Warn("using_default_http_port", "port", httpPort)
	}

	localIP := utils.GetLocalIP()
	log.Info("local_ip_detected", "ip", localIP)

	udpBridge := bridge.NewBridge(logger.WithContext("component", "bridge"))
	udpBridge.Start()
	defer udpBridge.Stop()

	server := udp.NewServer(port)
	if err := server.Start(); err != nil {
		log.Error("failed_to_start_udp_server",
			"error", err.Error(),
			"port", port)
		os.Exit(1)
	}
	defer server.Stop()

	log.Info("udp_server_running",
		"port", port,
		"pid", os.Getpid())

	// Start HTTP server for SSE endpoint
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Cache-Control"}
	config.ExposeHeaders = []string{"Content-Type"}
	router.Use(cors.New(config))

	// SSE endpoint
	router.GET("/events", server.GetSSEBroker().ServeSSE)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":      "ok",
			"udp_port":    port,
			"http_port":   httpPort,
			"sse_clients": server.GetSSEBroker().GetClientCount(),
		})
	})

	// Start HTTP server in goroutine
	go func() {
		log.Info("http_sse_server_starting", "port", httpPort, "url", fmt.Sprintf("http://%s:%s", localIP, httpPort))
		if err := router.Run("0.0.0.0:" + httpPort); err != nil {
			log.Error("failed_to_start_http_server", "error", err.Error())
		}
	}()

	fmt.Printf("\n🚀 UDP Notification Server:\n")
	fmt.Printf("   UDP Port:      %s\n", port)
	fmt.Printf("   HTTP/SSE Port: %s\n", httpPort)
	fmt.Printf("   SSE Endpoint:  http://%s:%s/events\n", localIP, httpPort)
	fmt.Printf("   Health Check:  http://%s:%s/health\n\n", localIP, httpPort)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	log.Info("shutting_down_udp_server", "signal", sig.String())
}
