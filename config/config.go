package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// AppConfig holds all configuration for the application
type AppConfig struct {
	Discord struct {
		AppID    string `koanf:"app_id"`
		BotToken string `koanf:"bot_token"`
	} `koanf:"discord"`

	Dero struct {
		Username string `koanf:"username"`
		Password string `koanf:"password"`
	} `koanf:"dero"`

	Database struct {
		Directory string `koanf:"directory"`
	} `koanf:"database"`
}

// Global singleton config instance
var (
	cfg  *AppConfig
	once sync.Once
)

// Get returns the global AppConfig instance
func Get() *AppConfig {
	once.Do(func() {
		var err error
		cfg, err = load()
		if err != nil {
			slog.Error("Failed to load configuration", "error", err)
			os.Exit(1)
		}
	})
	return cfg
}

// Load configuration from various sources with proper precedence
func load() (*AppConfig, error) {
	k := koanf.New(".")

	// Default configuration
	defaultConfig := map[string]interface{}{
		"discord.app_id":     "1349959098543767602",
		"database.directory": "./dbfiles",
	}
	if err := k.Load(confmap.Provider(defaultConfig, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to load default config: %w", err)
	}

	// Configuration file - try multiple locations with fallbacks
	configLocations := []string{
		"/etc/app/config.yaml",            // Standard system location
		"/config/config.yaml",             // Docker mounted volume location
		filepath.Join(".", "config.yaml"), // Local file in current directory
	}

	configLoaded := false
	for _, loc := range configLocations {
		if _, err := os.Stat(loc); err == nil {
			slog.Info("Loading configuration file", "path", loc)
			if err := k.Load(file.Provider(loc), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("error loading config file %s: %w", loc, err)
			}
			configLoaded = true
			break
		}
	}

	if !configLoaded {
		slog.Warn("No config file found in any of the expected locations",
			"searched_locations", configLocations)
	}

	// Environment variables (highest priority)
	// Format: APP_DISCORD_BOT_TOKEN -> discord.bot_token
	callback := func(s string) string {
		// Convert APP_DISCORD_BOT_TOKEN to discord.bot_token
		s = strings.Replace(strings.ToLower(s), "app_", "", 1)
		s = strings.Replace(s, "_", ".", -1)
		return s
	}

	if err := k.Load(env.Provider("APP_", ".", callback), nil); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Create config instance
	var cfg AppConfig
	decoderConfig := koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &cfg,
		},
	}

	if err := k.UnmarshalWithConf("", &cfg, decoderConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Log configuration details (with sensitive information redacted)
	slog.Debug("Configuration loaded",
		"database_directory", cfg.Database.Directory,
		"discord_app_id", cfg.Discord.AppID,
		"bot_token_present", cfg.Discord.BotToken != "",
		"dero_credentials_present", cfg.Dero.Username != "" && cfg.Dero.Password != "")

	// Validate required configurations
	if cfg.Discord.BotToken == "" {
		return nil, fmt.Errorf("discord.bot_token is required")
	}

	if cfg.Dero.Username == "" || cfg.Dero.Password == "" {
		return nil, fmt.Errorf("dero credentials are required")
	}

	return &cfg, nil
}
