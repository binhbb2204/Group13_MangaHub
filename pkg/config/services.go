package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

type Service struct {
	Host     string
	Port     string
	Protocol string
}

type ServicesConfig struct {
	LocalIP   string
	API       Service
	WebSocket Service
	TCP       Service
	UDP       Service
	GRPC      Service
}

func LoadServicesConfig() *ServicesConfig {
	localIP := utils.GetLocalIP()

	cfg := &ServicesConfig{
		LocalIP: localIP,
		API: Service{
			Host:     getEnvOrDefault("API_HOST", localIP),
			Port:     getEnvOrDefault("API_PORT", "8080"),
			Protocol: "http",
		},
		WebSocket: Service{
			Host:     getEnvOrDefault("WEBSOCKET_HOST", localIP),
			Port:     getEnvOrDefault("WEBSOCKET_PORT", "9093"),
			Protocol: "ws",
		},
		TCP: Service{
			Host:     getEnvOrDefault("TCP_HOST", localIP),
			Port:     getEnvOrDefault("TCP_PORT", "9090"),
			Protocol: "tcp",
		},
		UDP: Service{
			Host:     getEnvOrDefault("UDP_HOST", localIP),
			Port:     getEnvOrDefault("UDP_PORT", "9091"),
			Protocol: "udp",
		},
		GRPC: Service{
			Host:     getEnvOrDefault("GRPC_HOST", localIP),
			Port:     getEnvOrDefault("GRPC_PORT", "50051"),
			Protocol: "http",
		},
	}

	return cfg
}

func (s *Service) URL() string {
	if s.Protocol == "tcp" || s.Protocol == "udp" {
		return fmt.Sprintf("%s:%s", s.Host, s.Port)
	}
	return fmt.Sprintf("%s://%s:%s", s.Protocol, s.Host, s.Port)
}

func (cfg *ServicesConfig) GetDiscoveryResponse() map[string]interface{} {
	return map[string]interface{}{
		"local_ip": cfg.LocalIP,
		"services": map[string]interface{}{
			"api":       cfg.API.URL(),
			"websocket": cfg.WebSocket.URL(),
			"grpc":      cfg.GRPC.URL(),
			"tcp":       cfg.TCP.URL(),
			"udp":       cfg.UDP.URL(),
		},
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func GetEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
