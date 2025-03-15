package discord

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// BotConfig contains configuration for the bot.
type BotConfig struct {
	AppID    string
	BotToken string
}

// Bot encapsulates the discordgo session and configuration.
type Bot struct {
	session *discordgo.Session
	config  BotConfig
	logger  *slog.Logger
}

// NewBot creates a new Bot instance, opens a websocket session, registers a global slash command,
// logs all incoming messages and interactions, and sends a greeting message to every guild.
func NewBot(cfg BotConfig) (*Bot, error) {
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}

	// Set necessary intents.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	bot := &Bot{
		session: dg,
		config:  cfg,
		logger:  slog.Default(),
	}

	// Register event handlers.
	dg.AddHandler(bot.onMessageCreate)
	dg.AddHandler(bot.onInteractionCreate)

	// Open the websocket connection.
	if err := dg.Open(); err != nil {
		return nil, err
	}

	// Register the global slash command "sayhi".
	_, err = dg.ApplicationCommandCreate(cfg.AppID, "", &discordgo.ApplicationCommand{
		Name:        "sayhi",
		Description: "Say hi to the bot",
	})
	if err != nil {
		bot.logger.Error("failed to create slash command", "command", "sayhi", "error", err)
	}

	// List all guilds (builds) and send a greeting message to each guild's system channel (if available).
	guilds := dg.State.Guilds
	for _, g := range guilds {
		bot.logger.Info("bot presence detected in guild", "guild_name", g.Name, "guild_id", g.ID)

		if g.SystemChannelID != "" {
			_, err = dg.ChannelMessageSend(g.SystemChannelID, "Bot is online! Use /sayhi to interact with me.")
			if err != nil {
				bot.logger.Error("failed to send greeting message",
					"guild_id", g.ID,
					"channel_id", g.SystemChannelID,
					"error", err)
			}
		} else {
			bot.logger.Warn("unable to send greeting, no system channel configured",
				"guild_id", g.ID,
				"guild_name", g.Name)
		}
	}

	return bot, nil
}

// onMessageCreate logs every message the bot sees (ignoring its own).
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	b.logger.Debug("message received",
		"author", m.Author.Username,
		"author_id", m.Author.ID,
		"channel_id", m.ChannelID,
		"content", m.Content,
		"attachments", len(m.Attachments))
}

// onInteractionCreate logs each interaction and responds to the "sayhi" command.
func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var username string
	var userID string

	if i.Member != nil && i.Member.User != nil {
		username = i.Member.User.Username
		userID = i.Member.User.ID
	} else if i.User != nil {
		username = i.User.Username
		userID = i.User.ID
	} else {
		username = "unknown"
		userID = "unknown"
	}

	// Use ApplicationCommandData() to obtain command-specific data.
	cmdData := i.ApplicationCommandData()

	b.logger.Info("interaction received",
		"username", username,
		"user_id", userID,
		"command", cmdData.Name,
		"interaction_id", i.ID,
		"guild_id", i.GuildID)

	if cmdData.Name == "sayhi" {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Hi there!",
			},
		})

		if err != nil {
			b.logger.Error("failed to respond to interaction",
				"command", "sayhi",
				"user_id", userID,
				"interaction_id", i.ID,
				"error", err)
		}
	}
}

// Close gracefully closes the Discord session.
func (b *Bot) Close() error {
	b.logger.Info("shutting down bot")
	return b.session.Close()
}
