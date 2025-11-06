package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct{
	Server struct{
		Host          string `yaml:"host"`
        HTTPPort      int    `yaml:"http_port"`
        TCPPort       int    `yaml:"tcp_port"`
        UDPPort       int    `yaml:"udp_port"`
        GRPCPort      int    `yaml:"grpc_port"`
        WebSocketPort int    `yaml:"websocket_port"`
	}
	Database struct{
        Path string `yaml:"path"`
    } `yaml:"database"`
	User struct{
        Username string `yaml:"username"`
        Token    string `yaml:"token"`
    } `yaml:"user"`
	Sync struct{
        AutoSync           bool   `yaml:"auto_sync"`
        ConflictResolution string `yaml:"conflict_resolution"`
    } `yaml:"sync"`
    Notifications struct{
        Enabled bool `yaml:"enabled"`
        Sound   bool `yaml:"sound"`
    } `yaml:"notifications"`
    Logging struct{
        Level string `yaml:"level"`
        Path  string `yaml:"path"`
    } `yaml:"logging"`
}

var GlobalConfig *Config

func GetConfigDir() (string, error){
	home, err := os.UserHomeDir()
	if err != nil{
		return "", err
	}
	return filepath.Join(home, ".mangahub"), nil
}

func GetConfigPath() (string, error){
	configDir, err := GetConfigDir()
    if err != nil {
        return "", err
    }
	return filepath.Join(configDir, "config.yaml"), nil
}

func Load() (*Config, error){
	configPath, err := GetConfigPath()
	if err != nil{
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil{
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil{
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	GlobalConfig = &config
	return &config, nil
}

func Save(config *Config) error{
	configPath, err := GetConfigPath()
	if err != nil{
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil{
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil{
		return fmt.Errorf("failed to write config file: %w", err)
	}

	GlobalConfig = config
	return nil
}

func Init() error{
	configDir, err := GetConfigDir()
	if err != nil{
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil{
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	dataDir := filepath.Join(configDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil{
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	logsDir := filepath.Join(configDir, "logs")
	if err := os.Mkdir(logsDir, 0755); err != nil{
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	config := &Config{}
	config.Server.Host = "localhost"
	config.Server.HTTPPort = 8080
    config.Server.TCPPort = 9090
    config.Server.UDPPort = 9091
    config.Server.GRPCPort = 9092
    config.Server.WebSocketPort = 9093
    config.Database.Path = filepath.Join(configDir, "data.db")
    config.User.Username = ""
    config.User.Token = ""
    config.Sync.AutoSync = true
    config.Sync.ConflictResolution = "last_write_wins"
    config.Notifications.Enabled = true
    config.Notifications.Sound = false
    config.Logging.Level = "info"
    config.Logging.Path = logsDir

	return Save(config)
}

func UpdateUserToken(username, token string) error{
	config, err := Load()
	if err != nil{
		return err
	}

	config.User.Username = username
	config.User.Token = token

	return Save(config)
}

func ClearUserToken() error{
	config, err := Load()
	if err != nil{
		return err
	}

	config.User.Username = ""
	config.User.Token = ""

	return Save(config)
}

func GetServerURL() (string, error){
	config, err := Load()
	if err != nil{
		return "", err
	}
	return fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.HTTPPort), nil
}
