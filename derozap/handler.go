package derozap

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

// convertDateFormat converts a date from "yyyy/mm/dd" to "MM/DD/YYYY" format,
// which is the format expected by the Dero ZAP library.
func convertDateFormat(dateStr string) (string, error) {
	t, err := time.Parse("2006/01/02", dateStr)
	if err != nil {
		return "", err
	}
	return t.Format("01/02/2006"), nil
}

// handleDerozapCommand processes the Discord command to fetch tag reads from Dero ZAP.
// It returns an embed with an ASCII grid showing months down the side and years (max 5) across the top.
func (c *Client) handleDerozapCommand(req DerozapRequest) (*discordgo.InteractionResponseData, error) {

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
	tagReads, err := c.FetchTagReads(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tag reads: %w", err)
	}

	// Aggregate counts by year and month.
	// yearMonthCounts[year][month] = count, where month is 1-12.
	yearMonthCounts := make(map[int]map[time.Month]int)
	for _, tr := range tagReads {
		// Parse using the required format "2006-01-02"
		t, err := time.Parse("2006-01-02", tr.Date)
		if err != nil {
			// Skip if date parsing fails.
			continue
		}

		yr := t.Year()
		mn := t.Month()

		if yearMonthCounts[yr] == nil {
			yearMonthCounts[yr] = make(map[time.Month]int)
		}
		yearMonthCounts[yr][mn]++
	}

	// Collect and sort the years.
	var years []int
	for y := range yearMonthCounts {
		years = append(years, y)
	}
	sort.Ints(years)

	// Limit to a maximum of 5 years (show the most recent 5 if available).
	if len(years) > 5 {
		years = years[len(years)-5:]
	}

	// Define month labels (short form).
	monthLabels := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

	// Helper function to pad strings to a fixed width.
	const colWidth = 8
	pad := func(s string, width int) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	var lines []string

	// Build header row: "Month" + each year.
	header := pad("Month", colWidth)
	for _, y := range years {
		header += pad(strconv.Itoa(y), colWidth)
	}
	lines = append(lines, header)

	// Build each row: row header is the month name, then counts for each year.
	for i, label := range monthLabels {
		row := pad(label, colWidth)
		monthNum := time.Month(i + 1)
		for _, y := range years {
			count := yearMonthCounts[y][monthNum]
			row += pad(strconv.Itoa(count), colWidth)
		}
		lines = append(lines, row)
	}

	// Combine lines into an ASCII grid inside a code block.
	asciiTable := "```\n" + strings.Join(lines, "\n") + "\n```"
	description := fmt.Sprintf("Total tag reads: %d\n\n%s", len(tagReads), asciiTable)

	embed := &discordgo.MessageEmbed{
		Title:       "Dero ZAP Tag Reads Detailed Breakdown",
		Description: description,
		Color:       0x00FF00, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	}, nil
}

// DiscordCommandRetrieveZaps returns the command handler for retrieving Dero ZAP tag reads.
func (c *Client) DiscordCommandRetrieveZaps() discord.BotFunctionI {
	return discord.NewBotFunction("retreive_zaps", c.handleDerozapCommand, nil)
}
