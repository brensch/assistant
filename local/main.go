package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/brensch/assistant/discord"
	"github.com/bwmarrin/discordgo"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
)

// CoolRequest defines the request structure for the "cool" command.
type CoolRequest struct {
	Message string
}

// BoolismRequest defines the request structure for the "boolism" command.
type BoolismRequest struct {
	Flag bool
}

// coolHandler processes a CoolRequest.
// It returns a pointer to InteractionResponseData that will be sent as the interaction response.
func coolHandler(req CoolRequest) (*discordgo.InteractionResponseData, error) {
	slog.Info("coolHandler executed", "message", req.Message)
	return &discordgo.InteractionResponseData{
		Content: "Cool command executed with message: " + req.Message,
	}, nil
}

// boolismHandler processes a BoolismRequest.
func boolismHandler(req BoolismRequest) (*discordgo.InteractionResponseData, error) {
	slog.Info("boolismHandler executed", "flag", req.Flag)
	return &discordgo.InteractionResponseData{
		Content: fmt.Sprintf("Boolism command executed with flag: %v", req.Flag),
	}, nil
}

func main() {
	// Configure pretty colored logging with tint.
	handler := tint.NewHandler(colorable.NewColorableStdout(), &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: "15:04:05.000",
		AddSource:  true,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Log startup message.
	slog.Info("Discord Bot Starting")

	// Get bot token from environment.
	botToken := os.Getenv("BOTTOKEN")
	if botToken == "" {
		slog.Error("BOTTOKEN environment variable not set")
		os.Exit(1)
	}

	// Configure and start the bot.
	cfg := discord.BotConfig{
		AppID:    "1349959098543767602",
		BotToken: botToken,
	}

	slog.Info("Initializing bot", "app_id", cfg.AppID, "token_prefix", botToken[:5]+"...")

	// Create a slice of bot functions using generics.
	functions := []discord.BotFunctionI{
		// The autocomplete parameter is nil here.
		discord.NewBotFunction("cool", coolHandler, nil),
		discord.NewBotFunction("boolism", boolismHandler, nil),
	}

	// Create the bot, providing the configuration and list of functions.
	bot, err := discord.NewBot(cfg, functions)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	// Log successful startup.
	slog.Info("Bot is now running")

	// Wait for an interrupt signal to gracefully shut down.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	// Shut down the bot.
	slog.Info("Shutting down bot...")
	if err := bot.Close(); err != nil {
		slog.Error("Error during shutdown", "error", err)
	}
}
