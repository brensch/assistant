package assistant

import (
	"log/slog"

	"github.com/brensch/assistant/discord"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {
	slog.Info("registering functions")
	functions.HTTP("discordHandler", discord.Handler)
}
