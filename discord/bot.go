package discord

import (
	"fmt"
	"strings"

	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// Bot encapsulates the discordgo session, configuration, and registered functions.
type Bot struct {
	session   *discordgo.Session
	config    BotConfig
	functions []BotFunctionI
}

// BotConfig contains configuration for the bot.
type BotConfig struct {
	AppID    string
	BotToken string
}

// NewBot creates a new Bot instance, registers each command function globally,
// and sends an online message listing all available commands to each guild.
func NewBot(cfg BotConfig, functions []BotFunctionI) (*Bot, error) {
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}

	// Set necessary intents.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	bot := &Bot{
		session:   dg,
		config:    cfg,
		functions: functions,
	}

	// Register event handlers.
	dg.AddHandler(bot.onMessageCreate)
	dg.AddHandler(bot.onInteractionCreate)

	// Open the websocket connection.
	if err := dg.Open(); err != nil {
		return nil, err
	}

	// Build a comma-separated list of command names for the online message.
	var availableCommands []string
	for _, fn := range functions {
		availableCommands = append(availableCommands, fn.GetName())
	}
	commandsMessage := fmt.Sprintf("I'm online! Available commands: %s", strings.Join(availableCommands, ", "))

	// Register each command globally by using an empty guild ID.
	for _, fn := range functions {
		options, err := structToCommandOptions(fn.GetRequestPrototype())
		if err != nil {
			slog.Error("failed to generate command options", "command", fn.GetName(), "error", err)
			return nil, err
		}
		slog.Debug("initialising function", "name", fn.GetName(), "options", options)
		cmd := &discordgo.ApplicationCommand{
			Name:        fn.GetName(),
			Description: "Auto-generated command for " + fn.GetName(),
			Options:     options,
		}
		// Pass an empty string as the guild ID for global registration.
		_, err = dg.ApplicationCommandCreate(cfg.AppID, "", cmd)
		if err != nil {
			slog.Error("failed to create global slash command", "command", fn.GetName(), "error", err)
			return nil, err
		}
	}

	// Send the online message to every guild the bot is in.
	for _, guild := range dg.State.Guilds {
		// Retrieve the guild channels.
		channels, err := dg.GuildChannels(guild.ID)
		if err != nil {
			slog.Error("failed to get guild channels", "guild", guild.ID, "error", err)
			continue // Skip sending the message if channels cannot be fetched.
		}

		// Find the first text channel.
		var targetChannel string
		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildText {
				targetChannel = channel.ID
				break
			}
		}

		// If a text channel is found, send the online message.
		if targetChannel != "" {
			_, err = dg.ChannelMessageSend(targetChannel, commandsMessage)
			if err != nil {
				slog.Error("failed to send online message", "guild", guild.ID, "error", err)
			}
		}
	}

	return bot, nil
}

// onMessageCreate logs every message the bot sees (ignoring its own).
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	slog.Debug("message received",
		"author", m.Author.Username,
		"author_id", m.Author.ID,
		"channel_id", m.ChannelID,
		"content", m.Content,
		"attachments", len(m.Attachments))
}

// onInteractionCreate routes interactions to the correct BotFunction based on the command name.
func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cmdData := i.ApplicationCommandData()

	slog.Debug("received interaction", "cmd", cmdData)

	// Find the registered function with a matching name.
	var fn BotFunctionI
	for _, f := range b.functions {
		if f.GetName() == cmdData.Name {
			fn = f
			break
		}
	}
	if fn == nil {
		slog.Warn("received unknown command", "command", cmdData.Name)
		return
	}

	// Execute the function's handler using the interaction data.
	respData, err := fn.HandleInteraction(&cmdData)
	if err != nil {
		slog.Error("failed to execute command", "command", fn.GetName(), "error", err)
		return
	}

	// Respond to the interaction using the returned response data.
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: respData,
	})
	if err != nil {
		slog.Error("failed to respond to command", "command", fn.GetName(), "error", err)
	}
}

// Close gracefully closes the Discord session.
func (b *Bot) Close() error {
	slog.Info("shutting down bot")
	return b.session.Close()
}
