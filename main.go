package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/brensch/assistant/config"
	"github.com/brensch/assistant/db"
	"github.com/brensch/assistant/derozap"
	"github.com/brensch/assistant/discord"
	"github.com/brensch/assistant/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure pretty colored logging with tint.
	opts := log.PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler := log.NewPrettyHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Log startup message.
	slog.Info("Discord Bot Starting")

	// Load configuration
	cfg := config.Get()
	slog.Info("Configuration loaded successfully")

	// Create the database directory if it doesn't exist
	dbDir := cfg.Database.Directory
	os.MkdirAll(dbDir, 0755)

	// Try using in-memory mode first to test if DuckDB works
	dbClient, err := db.NewClient(dbDir)
	if err != nil {
		slog.Error("failed to create client", "error", err)
		os.Exit(1)
	}

	// Start the client.
	err = dbClient.Start(ctx)
	if err != nil {
		slog.Error("failed to start client", "error", err)
		os.Exit(1)
	}

	// Create our example table if it doesn't exist
	_, err = dbClient.Conn().Exec("CREATE TABLE IF NOT EXISTS example(id INTEGER, name VARCHAR)")
	if err != nil {
		slog.Error("failed to create table", "error", err)
		os.Exit(1)
	}

	// Configure and start the bot using config values
	discordCfg := discord.BotConfig{
		AppID:    cfg.Discord.AppID,
		BotToken: cfg.Discord.BotToken,
	}

	slog.Info("Initializing bot", "app_id", discordCfg.AppID, "token_prefix", discordCfg.BotToken[:5]+"...")

	// Use config values for DERO client
	deroClient, err := derozap.NewClient(cfg.Dero.Username, cfg.Dero.Password, dbClient)
	if err != nil {
		slog.Error("failed to init dero zap", "err", err)
		os.Exit(1)
	}

	// Create a slice of bot functions using generics.
	functions := []discord.BotFunctionI{
		// The autocomplete parameter is nil here.
		deroClient.DiscordFunctionRetrieveZaps(),
	}

	// Define scheduled tasks
	schedules := []discord.BotScheduleI{
		deroClient.DiscordScheduleZapCheck("0 * * * *"),
	}

	// Create the bot, providing the configuration and list of functions.
	bot, err := discord.NewBot(discordCfg, functions, schedules)
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

	err = dbClient.Stop()
	if err != nil {
		slog.Error("failed to stop client", "error", err)
	}

	// Shut down the bot.
	slog.Info("Shutting down bot...")
	err = bot.Close()
	if err != nil {
		slog.Error("Error during shutdown", "error", err)
	}

	cancel()
}
