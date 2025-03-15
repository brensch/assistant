package main

import (
	"log/slog"
	"os"
	"os/signal"

	"github.com/brensch/assistant/discord"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
)

func main() {
	// Configure pretty colored logging with tint
	handler := tint.NewHandler(colorable.NewColorableStdout(), &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: "15:04:05.000",
		AddSource:  true,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Log startup message
	slog.Info("Discord Bot Starting")

	// Get bot token from environment
	botToken := os.Getenv("BOTTOKEN")
	if botToken == "" {
		slog.Error("BOTTOKEN environment variable not set")
		os.Exit(1)
	}

	// Configure and start the bot
	cfg := discord.BotConfig{
		AppID:    "1349959098543767602",
		BotToken: botToken,
	}

	slog.Info("Initializing bot", "app_id", cfg.AppID, "token_prefix", botToken[:5]+"...")

	bot, err := discord.NewBot(cfg)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	// Log successful startup
	slog.Info("Bot is now running")

	// Wait for interrupt signal to gracefully shut down
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	// Shut down the bot
	slog.Info("Shutting down bot...")
	if err := bot.Close(); err != nil {
		slog.Error("Error during shutdown", "error", err)
	}
}
