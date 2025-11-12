package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
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

	log := logger.GetLogger().WithContext("component", "udp_main")
	log.Info("starting_udp_server", "version", "1.0.0")

	port := os.Getenv("UDP_PORT")
	if port == "" {
		port = "9091"
		log.Warn("using_default_port", "port", port)
	}

	udpBridge := bridge.NewBridge(logger.WithContext("component", "bridge"))
	udpBridge.Start()
	defer udpBridge.Stop()

	server := udp.NewServer(port, udpBridge)
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	log.Info("shutting_down_udp_server", "signal", sig.String())
}
