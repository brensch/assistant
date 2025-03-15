package assistant

import (
	"log/slog"
	"os"

	"github.com/brensch/assistant/discord"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {

	botToken := os.Getenv("BOTTOKEN")

	cfg := discord.BotConfig{
		AppID:    "1349959098543767602",
		BotToken: botToken,
		// ChannelIDs: []string{"channelID1", "channelID2"},
	}

	bot, err := discord.NewBot(cfg)
	if err != nil {
		// In Cloud Functions, log.Fatal will cause a startup failure.
		panic(err)
	}
	slog.Info("registering functions")
	functions.HTTP("discordHandler", bot.Handler)
}
