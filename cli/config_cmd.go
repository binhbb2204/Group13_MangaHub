package cli

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and modify MangaHub CLI configuration.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		fmt.Println("Current Configuration:")
		fmt.Println("----------------------")

		v := reflect.ValueOf(*cfg)
		t := v.Type()

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			typeField := t.Field(i)

			fmt.Printf("[%s]\n", typeField.Name)
			if field.Kind() == reflect.Struct {
				for j := 0; j < field.NumField(); j++ {
					subField := field.Field(j)
					subTypeField := field.Type().Field(j)
					tag := subTypeField.Tag.Get("yaml")
					if tag == "" {
						tag = subTypeField.Name
					}
					fmt.Printf("  %s: %v\n", tag, subField.Interface())
				}
			}
			fmt.Println()
		}

		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Long:  `Set a configuration value. Key should be in format 'section.key' (e.g., logging.level).`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			return err
		}

		parts := strings.Split(key, ".")
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format. Use 'section.key'")
		}

		section := strings.ToLower(parts[0])
		k := strings.ToLower(parts[1])

		updated := false

		switch section {
		case "server":
			switch k {
			case "host":
				cfg.Server.Host = value
				updated = true
			case "http_port":
				if v, err := strconv.Atoi(value); err == nil {
					cfg.Server.HTTPPort = v
					updated = true
				} else {
					return fmt.Errorf("invalid integer for http_port")
				}
			case "tcp_port":
				if v, err := strconv.Atoi(value); err == nil {
					cfg.Server.TCPPort = v
					updated = true
				} else {
					return fmt.Errorf("invalid integer for tcp_port")
				}
			case "udp_port":
				if v, err := strconv.Atoi(value); err == nil {
					cfg.Server.UDPPort = v
					updated = true
				} else {
					return fmt.Errorf("invalid integer for udp_port")
				}
			}
		case "sync":
			switch k {
			case "auto_sync":
				if v, err := strconv.ParseBool(value); err == nil {
					cfg.Sync.AutoSync = v
					updated = true
				} else {
					return fmt.Errorf("invalid boolean for auto_sync")
				}
			case "conflict_resolution":
				cfg.Sync.ConflictResolution = value
				updated = true
			}
		case "notifications":
			switch k {
			case "enabled":
				if v, err := strconv.ParseBool(value); err == nil {
					cfg.Notifications.Enabled = v
					updated = true
				} else {
					return fmt.Errorf("invalid boolean for enabled")
				}
			case "sound":
				if v, err := strconv.ParseBool(value); err == nil {
					cfg.Notifications.Sound = v
					updated = true
				} else {
					return fmt.Errorf("invalid boolean for sound")
				}
			}
		case "logging":
			switch k {
			case "level":
				cfg.Logging.Level = value
				updated = true
			}
		}

		if !updated {
			return fmt.Errorf("unknown configuration key: %s", key)
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		printSuccess(fmt.Sprintf("Updated %s to %s", key, value))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}
