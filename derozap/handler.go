package derozap

import (
	"errors"
	"fmt"
	"time"

	"github.com/brensch/assistant/discord"

	"github.com/bwmarrin/discordgo"
)

// DerozapRequest defines the expected inputs for the derozap command.
// The "start" and "end" dates are optional and must be in the format yyyy/mm/dd (e.g., 2025/03/14).
type DerozapRequest struct {
	Start string `discord:"optional,description=Optional start date in yyyy/mm/dd format (e.g., 2025/03/14)"`
	End   string `discord:"optional,description=Optional end date in yyyy/mm/dd format (e.g., 2025/03/14)"`
}

// convertDateFormat converts a date from "yyyy/mm/dd" to "MM/DD/YYYY" format
// which is the format expected by the Dero ZAP library.
func convertDateFormat(dateStr string) (string, error) {
	t, err := time.Parse("2006/01/02", dateStr)
	if err != nil {
		return "", err
	}
	return t.Format("01/02/2006"), nil
}

// handleDerozapCommand processes the Discord command to fetch tag reads from Dero ZAP.
func handleDerozapCommand(req DerozapRequest) (*discordgo.InteractionResponseData, error) {
	// Initialize the Dero ZAP client.
	// Replace "your_username" and "your_password" with actual credentials.
	client, err := NewClient("your_username", "your_password")
	if err != nil {
		return nil, fmt.Errorf("failed to create Dero ZAP client: %w", err)
	}

	// Prepare optional date range parameters.
	var options []ReportOption
	if req.Start != "" || req.End != "" {
		if req.Start == "" || req.End == "" {
			return nil, errors.New("both start and end dates must be provided if one is specified")
		}

		startFormatted, err := convertDateFormat(req.Start)
		if err != nil {
			return nil, fmt.Errorf("invalid start date format: %w", err)
		}

		endFormatted, err := convertDateFormat(req.End)
		if err != nil {
			return nil, fmt.Errorf("invalid end date format: %w", err)
		}

		options = append(options, WithDateRange(startFormatted, endFormatted))
	}

	// Fetch tag reads from Dero ZAP.
	tagReads, err := client.FetchTagReads(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tag reads: %w", err)
	}

	// Create a response message summarizing the result.
	responseMessage := fmt.Sprintf("Fetched %d tag reads from Dero ZAP.", len(tagReads))
	return &discordgo.InteractionResponseData{
		Content: responseMessage,
	}, nil
}

// DerozapCommand registers the derozap command with the Discord bot interface.
// When the "derozap" command is invoked, it will decode any provided start and end date options,
// convert them to the expected format, and then fetch the report.
var DerozapCommand = discord.NewBotFunction("derozap", handleDerozapCommand, nil)
