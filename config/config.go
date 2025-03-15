package config

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// AppConfig holds all configuration for the application
type AppConfig struct {
	Discord struct {
		AppID    string `yaml:"app_id"`
		BotToken string `yaml:"bot_token"`
	} `yaml:"discord"`

	Dero struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"dero"`

	Database struct {
		Directory string `yaml:"directory"`
	} `yaml:"database"`
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

// Load configuration from .conf file
func load() (*AppConfig, error) {
	// Config file path
	configFile := ".conf"
	slog.Info("Loading configuration file", "path", configFile)

	// Read the configuration file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse the YAML
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
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
