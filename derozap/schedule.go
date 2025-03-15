package derozap

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/brensch/assistant/discord"
	"github.com/bwmarrin/discordgo"
)

// DiscordScheduleZapCheck returns a scheduled task that periodically checks for new Dero ZAP records
func (c *Client) DiscordScheduleZapCheck(cronExpression string) discord.BotScheduleI {
	return discord.NewBotSchedule("derozap_check", cronExpression, c.executeZapCheck)
}

// executeZapCheck is the handler for the scheduled task
// It fetches tag reads, stores new ones in the database, and returns an embed notification if new records are found
func (c *Client) executeZapCheck() (*discordgo.MessageEmbed, error) {
	slog.Info("Executing scheduled Derozap check")

	// Fetch tag reads
	tagReads, err := c.FetchTagReads()
	if err != nil {
		errorMsg := fmt.Sprintf("Error fetching tag reads: %v", err)
		slog.Error(errorMsg)
		return &discordgo.MessageEmbed{
			Title:       "Dero ZAP Check Error",
			Description: errorMsg,
			Color:       0xFF0000, // Red for errors
			Timestamp:   time.Now().Format(time.RFC3339),
		}, nil
	}

	if len(tagReads) == 0 {
		slog.Warn("No tags found")
		return nil, nil // No notification needed
	}

	// Store new tag reads in the database
	newRecords, err := c.storeNewTagReads(tagReads)
	if err != nil {
		slog.Error("Failed to store tag reads", "error", err)
		// Continue with Discord notification even if DB storage failed
	}

	// If no new records, don't send a notification
	if len(newRecords) == 0 {
		slog.Debug("No new tags found")
		return nil, nil
	}

	// Create description for notification
	description := fmt.Sprintf("Found %d new record(s) (%d entries total = $%d):\n",
		len(newRecords), len(tagReads), len(tagReads)*15)

	// Add details of the new records (limit to avoid overly long messages)
	maxToShow := 5
	if len(newRecords) < maxToShow {
		maxToShow = len(newRecords)
	}

	for i := 0; i < maxToShow; i++ {
		description += fmt.Sprintf("• %s: Tag ID %s\n", newRecords[i].Date, newRecords[i].TagID)
	}

	// Add ellipsis if more records were found than shown
	if len(newRecords) > maxToShow {
		description += fmt.Sprintf("• ... and %d more\n", len(newRecords)-maxToShow)
	}

	// Create and return the notification embed
	return &discordgo.MessageEmbed{
		Title:       "New Dero ZAPs Detected",
		Description: description,
		Color:       0x00FF00, // Green for success
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Automated Dero ZAP check",
		},
	}, nil
}
