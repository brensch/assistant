package discord

import (
	"fmt"

	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// getFirstTextChannel returns the ID of the first available text channel in the given guild.
// If no text channel is found, it returns an error.
func (b *Bot) getFirstTextChannel(guildID string) (string, error) {
	channels, err := b.session.GuildChannels(guildID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve channels for guild %s: %w", guildID, err)
	}

	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			return channel.ID, nil
		}
	}
	return "", fmt.Errorf("no text channel found in guild %s", guildID)
}

// SendMessage broadcasts a plain text message to the first available text channel
// in every guild that the bot is currently in.
func (b *Bot) SendMessage(content string) {
	// Iterate over all guilds in the bot's state.
	for _, guild := range b.session.State.Guilds {
		targetChannel, err := b.getFirstTextChannel(guild.ID)
		if err != nil {
			slog.Error("Error getting text channel", "guild", guild.ID, "error", err)
			continue
		}

		msg, err := b.session.ChannelMessageSend(targetChannel, content)
		if err != nil {
			slog.Error("Failed to send message", "guild", guild.ID, "channel", targetChannel, "error", err)
		} else {
			slog.Info("Message sent", "guild", guild.ID, "channel", targetChannel, "content", msg.Content)
		}
	}
}

// SendEmbed broadcasts an embed message to the first available text channel
// in every guild that the bot is currently in.
func (b *Bot) SendEmbed(embed *discordgo.MessageEmbed) {
	// Iterate over all guilds in the bot's state.
	for _, guild := range b.session.State.Guilds {
		targetChannel, err := b.getFirstTextChannel(guild.ID)
		if err != nil {
			slog.Error("Error getting text channel", "guild", guild.ID, "error", err)
			continue
		}

		_, err = b.session.ChannelMessageSendEmbed(targetChannel, embed)
		if err != nil {
			slog.Error("Failed to send embed", "guild", guild.ID, "channel", targetChannel, "error", err)
		} else {
			slog.Info("Embed sent", "guild", guild.ID, "channel", targetChannel)
		}
	}
}
