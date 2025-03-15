package discord

import (
	"fmt"
	"strings"

	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// Bot encapsulates the discordgo session, configuration, registered functions, and schedules.
type Bot struct {
	session         *discordgo.Session
	config          BotConfig
	functions       []BotFunctionI
	schedules       []BotScheduleI
	scheduleManager *scheduleManager
}

// BotConfig contains configuration for the bot.
type BotConfig struct {
	AppID    string
	BotToken string
}

// NewBot creates a new Bot instance, re-registers each command function on a per-guild basis,
// and sends an online message listing all available commands to each guild.
// It also initializes scheduled tasks based on the provided cron expressions.
func NewBot(cfg BotConfig, functions []BotFunctionI, schedules []BotScheduleI) (*Bot, error) {
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
		schedules: schedules,
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

	// Add schedule names to the message
	var activeSchedules []string
	for _, schedule := range schedules {
		activeSchedules = append(activeSchedules, fmt.Sprintf("%s (%s)", schedule.GetName(), schedule.GetCronExpression()))
	}

	commandsMessage := fmt.Sprintf("Assistant online. Available commands: %s", strings.Join(availableCommands, ", "))
	if len(activeSchedules) > 0 {
		commandsMessage += fmt.Sprintf("\nActive schedules: %s", strings.Join(activeSchedules, ", "))
	}

	// For each guild, delete all existing bot commands and register new ones.
	for _, guild := range dg.State.Guilds {
		// Retrieve the existing commands for the guild.
		existingCommands, err := dg.ApplicationCommands(cfg.AppID, guild.ID)
		if err != nil {
			slog.Error("failed to get commands for guild", "guild", guild.ID, "error", err)
			continue
		}
		// Delete each existing command.
		for _, cmd := range existingCommands {
			err := dg.ApplicationCommandDelete(cfg.AppID, guild.ID, cmd.ID)
			if err != nil {
				slog.Error("failed to delete command", "guild", guild.ID, "command", cmd.Name, "error", err)
			} else {
				slog.Debug("deleted command", "guild", guild.ID, "command", cmd.Name)
			}
		}
		// Register each new command for this guild.
		for _, fn := range functions {
			options, err := structToCommandOptions(fn.GetRequestPrototype())
			if err != nil {
				slog.Error("failed to generate command options", "command", fn.GetName(), "error", err)
				return nil, err
			}
			slog.Debug("initialising function", "name", fn.GetName(), "options", options)
			newCmd := &discordgo.ApplicationCommand{
				Name:        fn.GetName(),
				Description: "Auto-generated command for " + fn.GetName(),
				Options:     options,
			}
			_, err = dg.ApplicationCommandCreate(cfg.AppID, guild.ID, newCmd)
			if err != nil {
				slog.Error("failed to create guild slash command", "guild", guild.ID, "command", fn.GetName(), "error", err)
				return nil, err
			}
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

	// Initialize and start the schedule manager if there are schedules
	if len(schedules) > 0 {
		bot.scheduleManager = newScheduleManager(bot, schedules)
		err = bot.scheduleManager.start()
		if err != nil {
			slog.Error("failed to start schedule manager", "error", err)
			return nil, err
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
		// Optionally send an error embed for an unknown command.
		errorEmbed := &discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Unknown command: " + cmdData.Name,
			Color:       0xFF0000,
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{errorEmbed},
			},
		})
		return
	}

	// Execute the function's handler using the interaction data.
	respData, err := fn.HandleInteraction(&cmdData)
	if err != nil {
		slog.Error("failed to execute command", "command", fn.GetName(), "error", err.Error())
		errorEmbed := &discordgo.MessageEmbed{
			Title:       "Error",
			Description: fmt.Sprintf("```%v```", err),
			Color:       0xFF0000,
		}
		// Respond with the error embed.
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{errorEmbed},
			},
		})
		return
	}

	// Respond to the interaction using the returned response data.
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: respData,
	})
	if err != nil {
		slog.Error("failed to respond to command", "command", fn.GetName(), "error", err)
		errorEmbed := &discordgo.MessageEmbed{
			Title:       "Error",
			Description: fmt.Sprintf("```%v```", err),
			Color:       0xFF0000,
		}
		// Attempt to send a follow-up error message if the initial response fails.
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{errorEmbed},
		})
	}
}

// Close gracefully closes the Discord session and stops the schedule manager.
func (b *Bot) Close() error {
	slog.Info("shutting down bot")

	// Stop the schedule manager if it was initialized
	if b.scheduleManager != nil {
		b.scheduleManager.stop()
	}

	return b.session.Close()
}
