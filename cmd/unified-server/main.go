package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/grpc"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	grpc_server "google.golang.org/grpc"
)

type ServerOrchestrator struct {
	logger       *logger.Logger
	bridge       *bridge.UnifiedBridge
	tcpServer    *tcp.Server
	udpServer    *udp.Server
	wsServer     *websocket.Server
	grpcServer   *grpc.Server
	grpcListener *grpc_server.Server
	db           *sql.DB
	config       *Config
	stopChan     chan os.Signal
}

type Config struct {
	TCPPort       string
	UDPPort       string
	WebSocketPort string
	GRPCPort      string
	JWTSecret     string
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

	if o.config.EnableTCP {
		o.logger.Info("initializing_tcp_server", "port", o.config.TCPPort)
		o.tcpServer = tcp.NewServer(o.config.TCPPort, nil)
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

	errChan := make(chan error, 4)

	if o.config.EnableTCP && o.tcpServer != nil {
		go func() {
			o.logger.Info("starting_tcp_server", "port", o.config.TCPPort)
			if err := o.tcpServer.Start(); err != nil {
				o.logger.Error("tcp_server_start_failed", "error", err.Error())
				errChan <- fmt.Errorf("TCP server: %w", err)
			}
		}()
	}

	if o.config.EnableUDP && o.udpServer != nil {
		go func() {
			o.logger.Info("starting_udp_server", "port", o.config.UDPPort)
			if err := o.udpServer.Start(); err != nil {
				o.logger.Error("udp_server_start_failed", "error", err.Error())
				errChan <- fmt.Errorf("UDP server: %w", err)
			}
		}()
	}

	if o.config.EnableWS && o.wsServer != nil {
		go func() {
			o.logger.Info("ws_server_ready", "port", o.config.WebSocketPort)
		}()
	}

	if o.config.EnableGRPC && o.grpcServer != nil {
		go func() {
			o.logger.Info("grpc_server_ready", "port", o.config.GRPCPort)
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

	dbPath := getEnv("DB_PATH", "./data/manga_hub.db")
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
		TCPPort:       getEnv("TCP_PORT", "9090"),
		UDPPort:       getEnv("UDP_PORT", "9091"),
		WebSocketPort: getEnv("WS_PORT", "9093"),
		GRPCPort:      getEnv("GRPC_PORT", "9092"),
		JWTSecret:     jwtSecret,
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
