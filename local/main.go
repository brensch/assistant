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
	if botToken == "" {
		slog.Error("BOTTOKEN environment variable not set")
		os.Exit(1)
	}

	cfg := discord.BotConfig{
		AppID:    "1349959098543767602",
		BotToken: botToken,
		// Optionally, supply channel IDs if needed.
	}
	bot, err := discord.NewBot(cfg)
	if err != nil {
		panic(err)
	}

	_ = bot

	// Start the HTTP server in a goroutine.
	go func() {
		slog.Info("starting server on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("error starting server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for an interrupt signal to gracefully shut down.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("error during server shutdown", "error", err)
	}
	slog.Info("server gracefully stopped")
}
