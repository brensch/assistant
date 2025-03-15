package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/brensch/assistant/db"
	"github.com/brensch/assistant/derozap"
	"github.com/brensch/assistant/discord"
	"github.com/bwmarrin/discordgo"
)

// CoolRequest defines the request structure for the "cool" command.
type CoolRequest struct {
	Message string
}

// coolHandler processes a CoolRequest.
// It returns a pointer to InteractionResponseData that will be sent as the interaction response.
func coolHandler(req CoolRequest) (*discordgo.InteractionResponseData, error) {
	slog.Info("coolHandler executed", "message", req.Message)
	return &discordgo.InteractionResponseData{
		Content: "Cool command executed with message: " + req.Message,
	}, nil
}

// BoolismRequest defines the request structure for the "boolism" command.
type BoolismRequest struct {
	Flag  bool
	Color string `discord:"optional,description:Favorite color,choices:red|Red;blue|Blue;green|Green,default:blue"`
}

// boolismHandler processes a BoolismRequest.
func boolismHandler(req BoolismRequest) (*discordgo.InteractionResponseData, error) {
	slog.Info("boolismHandler executed", "flag", req.Flag)
	return &discordgo.InteractionResponseData{
		Content: fmt.Sprintf("Boolism command executed with flag: %v", req.Flag),
	}, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure pretty colored logging with tint.
	opts := PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler := NewPrettyHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Log startup message.
	slog.Info("Discord Bot Starting")

	// Create the database directory if it doesn't exist
	dbDir := "./dbfiles"
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

	// 1. Read all contents of the database first
	slog.Info("Reading existing data from database")
	rows, err := dbClient.Conn().QueryContext(ctx, "SELECT * FROM example")
	if err != nil {
		slog.Error("failed to query database", "error", err)
		os.Exit(1)
	}

	var existingData []struct {
		ID   int
		Name string
	}

	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			slog.Error("failed to scan row", "error", err)
			break
		}
		slog.Info("Existing data", "id", id, "name", name)
		existingData = append(existingData, struct {
			ID   int
			Name string
		}{id, name})
	}
	rows.Close()

	// 2. Write a new line to the database
	newID := len(existingData) + 1
	newName := fmt.Sprintf("Record-%d-%s", newID, time.Now().Format("15:04:05"))
	slog.Info("Writing new data to database", "id", newID, "name", newName)

	_, err = dbClient.Conn().ExecContext(ctx, "INSERT INTO example(id, name) VALUES(?, ?)", newID, newName)
	if err != nil {
		slog.Error("failed to insert data", "error", err)
		os.Exit(1)
	}

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

	deroZapUser := os.Getenv("DEROUSER")
	deroZapPass := os.Getenv("DEROPASS")

	deroClient, err := derozap.NewClient(deroZapUser, deroZapPass)
	if err != nil {
		slog.Error("failed to init dero zap", "err", err)
		os.Exit(1)
	}

	// Create a slice of bot functions using generics.
	functions := []discord.BotFunctionI{
		// The autocomplete parameter is nil here.
		discord.NewBotFunction("cool", coolHandler, nil),
		discord.NewBotFunction("boolism", boolismHandler, nil),
		deroClient.DiscordCommandRetrieveZaps(),
	}

	// Create the bot, providing the configuration and list of functions.
	bot, err := discord.NewBot(cfg, functions)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	// Log successful startup.
	slog.Info("Bot is now running")

	deroClient.Start(bot)

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
