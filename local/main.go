package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/brensch/assistant/discord"
)

func main() {
	// Configure logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Create a new HTTP server
	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	botToken := os.Getenv("BOTTOKEN")

	cfg := discord.BotConfig{
		AppID:    "1349959098543767602",
		BotToken: botToken,
		// ChannelIDs: []string{"channelID1", "channelID2"},
	}
	var err error
	bot, err := discord.NewBot(cfg)
	if err != nil {
		// In Cloud Functions, log.Fatal will cause a startup failure.
		panic(err)
	}

	// Register the Discord handler
	http.HandleFunc("/discord", bot.Handler)

	// Start the server in a goroutine
	go func() {
		slog.Info("starting server on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("error starting server", "error", err)
			os.Exit(1)
		}
	}()

	// Set up channel to handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Block until signal is received
	<-stop
	slog.Info("shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("error during server shutdown", "error", err)
	}

	slog.Info("server gracefully stopped")
}
