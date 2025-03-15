// Package derozap provides functionality to interact with the Dero ZAP system.
package derozap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brensch/assistant/db"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/net/html"
)

const (
	baseURL          = "https://www.derozap.com"
	loginEndpoint    = "/?s=login"
	reportEndpoint   = "/"
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	defaultStartDate = "01/01/2008"
	defaultEndDate   = "01/31/2100"
)

// TagRead represents a single RFID tag read from the system.
type TagRead struct {
	Date    string
	TagID   string
	RawData map[string]string // Additional data fields if present.
}

// Client represents a Dero ZAP client with authentication and session handling.
type Client struct {
	httpClient *http.Client
	dbClient   *db.Client
	username   string
	password   string
	loggedIn   bool
}

// NewClient creates a new Dero ZAP client.
func NewClient(username, password string, dbClient *db.Client) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		slog.Error("failed to create cookie jar", "error", err)
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &Client{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
		username: username,
		password: password,
		dbClient: dbClient,
	}

	// Create the table for storing DeroZAP reads if it doesn't exist
	err = client.createTagReadsTable()
	if err != nil {
		slog.Error("failed to create tag reads table", "error", err)
		return nil, fmt.Errorf("failed to create tag reads table: %w", err)
	}

	return client, nil
}

// createTagReadsTable creates the table for storing DeroZAP tag reads if it doesn't exist.
func (c *Client) createTagReadsTable() error {
	// SQL to create the table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS derozap_reads (
		zap_date DATE NOT NULL,
		tag_id TEXT NOT NULL,
		recorded_at TIMESTAMP NOT NULL,
		PRIMARY KEY (zap_date, tag_id)
	)
	`

	// Execute the SQL statement to create the table
	_, err := c.dbClient.Conn().Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create derozap_reads table: %w", err)
	}

	slog.Info("derozap_reads table created or already exists")
	return nil
}

// Login authenticates with the Dero ZAP service.
func (c *Client) Login() error {
	if c.loggedIn {
		return nil
	}

	loginURL := baseURL + loginEndpoint

	// Prepare form data.
	formData := url.Values{}
	formData.Set("email_login", c.username)
	formData.Set("password_login", c.password)

	// Create POST request.
	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(formData.Encode()))
	if err != nil {
		slog.Error("failed to create login request", "error", err)
		return fmt.Errorf("failed to create login request: %w", err)
	}

	// Set headers.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en-AU;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("Referer", baseURL+"/?s=login&a=logout")

	// Send the request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("login request failed", "error", err)
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check if login was successful - look for login failure indicators.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read login response", "error", err)
		return fmt.Errorf("failed to read login response: %w", err)
	}

	// Check for login success markers - look for welcome message.
	if strings.Contains(string(body), "Welcome,") {
		c.loggedIn = true
		return nil
	} else if strings.Contains(string(body), "Login failed") || strings.Contains(string(body), "Invalid login") {
		slog.Error("login failed: invalid credentials")
		return errors.New("login failed: invalid credentials")
	}

	// If we can't determine success/failure, check if redirected to dashboard.
	if resp.StatusCode == http.StatusOK &&
		(strings.Contains(resp.Request.URL.String(), "s=commuter") || !strings.Contains(resp.Request.URL.String(), "s=login")) {
		c.loggedIn = true
		return nil
	}

	slog.Error("login status uncertain, please check credentials", "response", string(body))
	return errors.New("login status uncertain, please check credentials")
}

// FetchTagReads retrieves tag reads from the report.
func (c *Client) FetchTagReads(options ...ReportOption) ([]TagRead, error) {
	if !c.loggedIn {
		err := c.Login()
		if err != nil {
			slog.Error("login failed in FetchTagReads", "error", err)
			return nil, err
		}
	}

	// Create default report parameters.
	params := defaultReportParams()

	// Apply any custom options.
	for _, option := range options {
		option(params)
	}

	// Build the full report URL.
	reportURL := buildReportURL(params)

	var allTagReads []TagRead
	currentPage := 1
	totalPages := 1 // Will be updated after first request.

	for currentPage <= totalPages {
		// Set page parameter for current request.
		pageURL := reportURL
		if currentPage > 1 {
			pageURL = fmt.Sprintf("%s&pg=%d", reportURL, currentPage)
		}

		// Make the request.
		req, err := http.NewRequest(http.MethodGet, pageURL, nil)
		if err != nil {
			slog.Error("failed to create report request", "error", err)
			return nil, fmt.Errorf("failed to create report request: %w", err)
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Referer", baseURL)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			slog.Error("failed to fetch report", "error", err)
			return nil, fmt.Errorf("failed to fetch report: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read report response", "error", err)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to read report response: %w", err)
		}
		resp.Body.Close()

		// Parse the results.
		tagReads, err := parseTagReads(body)
		if err != nil {
			slog.Error("failed to parse tag reads", "error", err)
			return nil, err
		}
		allTagReads = append(allTagReads, tagReads...)

		// Update total pages if this is the first page.
		if currentPage == 1 {
			totalPages = extractTotalPages(body)
		}

		currentPage++
	}

	return allTagReads, nil
}

// storeNewTagReads stores new tag reads in the database.
func (c *Client) storeNewTagReads(tagReads []TagRead) (int, error) {
	if len(tagReads) == 0 {
		return 0, nil
	}

	// Get the current timestamp
	now := time.Now()

	// Prepare to track how many new records were added
	newRecordsCount := 0

	// Process each tag read
	for _, tr := range tagReads {
		// Parse the date from the tag read
		zapDate, err := parseZapDate(tr.Date)
		if err != nil {
			slog.Error("failed to parse zap date", "date", tr.Date, "error", err)
			continue
		}

		// Format the date for SQL query
		formattedDate := zapDate.Format("2006-01-02") // SQL date format YYYY-MM-DD

		// Check if this record already exists in the database
		checkSQL := `SELECT COUNT(*) FROM derozap_reads WHERE zap_date = ? AND tag_id = ?`
		rows, err := c.dbClient.Conn().Query(checkSQL, formattedDate, tr.TagID)
		if err != nil {
			slog.Error("failed to check if tag read exists", "error", err, "tag_id", tr.TagID, "date", formattedDate)
			continue
		}

		var count int
		if rows.Next() {
			err = rows.Scan(&count)
			if err != nil {
				slog.Error("failed to scan count", "error", err)
				rows.Close()
				continue
			}
		}
		rows.Close()

		// If record doesn't exist, insert it
		if count == 0 {
			insertSQL := `INSERT INTO derozap_reads (zap_date, tag_id, recorded_at) VALUES (?, ?, ?)`
			_, err := c.dbClient.Conn().Exec(insertSQL, formattedDate, tr.TagID, now)
			if err != nil {
				slog.Error("failed to insert tag read", "error", err, "tag_id", tr.TagID, "date", formattedDate)
				continue
			}

			newRecordsCount++
			slog.Info("inserted new tag read", "tag_id", tr.TagID, "date", formattedDate)
		}
	}

	return newRecordsCount, nil
}

// parseZapDate parses a date string from the format in tag reads.
func parseZapDate(dateStr string) (time.Time, error) {
	// Assuming the date format is MM/DD/YYYY or similar
	// First, try common format
	date, err := time.Parse("01/02/2006", dateStr)
	if err == nil {
		return date, nil
	}

	// Try alternative formats if the first attempt fails
	formats := []string{
		"01/02/2006",
		"01/02/2006 03:04:05 PM",
		"01/02/2006 15:04:05",
		"2006-01-02",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		date, err := time.Parse(format, dateStr)
		if err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// ReportParams represents the parameters for a report request.
type ReportParams struct {
	DateRange      string // "al" for all.
	StartDate      string // Format: MM/DD/YYYY.
	EndDate        string // Format: MM/DD/YYYY.
	ReportSection  string // "commuter_report".
	ReportID       int    // 81.
	SortColumn     string // "ZAP Date".
	SortDirection  string // "asc" or "dec".
	ReportType     string // 1051 for Tag Reads by Date.
	ResultsPerPage int    // 50, 100, etc.
	MinZaps        int    // Minimum number of zaps to show.
}

// ReportOption is a function that modifies ReportParams.
type ReportOption func(*ReportParams)

// defaultReportParams returns default parameters for report requests.
func defaultReportParams() *ReportParams {
	return &ReportParams{
		DateRange:      "al",
		StartDate:      defaultStartDate,
		EndDate:        defaultEndDate,
		ReportSection:  "commuter_report",
		ReportID:       81,
		SortColumn:     "ZAP Date",
		SortDirection:  "dec",
		ReportType:     "1051",
		ResultsPerPage: 100,
		MinZaps:        1,
	}
}

// WithDateRange sets a custom date range for the report.
func WithDateRange(startDate, endDate string) ReportOption {
	return func(params *ReportParams) {
		params.StartDate = startDate
		params.EndDate = endDate
	}
}

// WithResultsPerPage sets the number of results per page.
func WithResultsPerPage(count int) ReportOption {
	return func(params *ReportParams) {
		params.ResultsPerPage = count
	}
}

// WithSortOrder sets the sort column and direction.
func WithSortOrder(column, direction string) ReportOption {
	return func(params *ReportParams) {
		params.SortColumn = column
		params.SortDirection = direction
	}
}

// buildReportURL creates the URL for fetching reports with the given parameters.
func buildReportURL(params *ReportParams) string {
	// URL encode sort column if it contains spaces.
	sortColumn := url.QueryEscape(params.SortColumn)

	url := fmt.Sprintf(
		"%s/?dr=%s&ds=%s&de=%s&s=%s&i=%d&sc=%s&sd=%s&sa=&rpid=%s&pp=%d&mz=%d",
		baseURL,
		params.DateRange,
		url.QueryEscape(params.StartDate),
		url.QueryEscape(params.EndDate),
		params.ReportSection,
		params.ReportID,
		sortColumn,
		params.SortDirection,
		params.ReportType,
		params.ResultsPerPage,
		params.MinZaps,
	)

	return url
}

// parseTagReads extracts tag read information from HTML response.
func parseTagReads(htmlBody []byte) ([]TagRead, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBody))
	if err != nil {
		slog.Error("failed to parse HTML", "error", err)
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var tagReads []TagRead

	// Find the table with class "reportTable".
	var findTable func(*html.Node) *html.Node
	findTable = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "table" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "reportTable") {
					return n
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found := findTable(c); found != nil {
				return found
			}
		}

		return nil
	}

	table := findTable(doc)
	if table == nil {
		slog.Error("report table not found in HTML response")
		return nil, errors.New("report table not found in HTML response")
	}

	// Parse the table rows.
	var processRow func(*html.Node)
	processRow = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var date, tagID string
			var cells []*html.Node

			// Collect the cell nodes.
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "td" {
					cells = append(cells, c)
				}
			}

			// Parse cells if we have at least 2 (date and tag ID).
			if len(cells) >= 2 {
				// Extract date from first cell.
				dateCell := cells[0]
				for c := dateCell.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						date = strings.TrimSpace(c.Data)
						break
					}
				}

				// Extract tag ID from second cell.
				tagCell := cells[1]
				for c := tagCell.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						tagID = strings.TrimSpace(c.Data)
						break
					}
				}

				// Create tag read if we have both values.
				if date != "" && tagID != "" {
					tagRead := TagRead{
						Date:    date,
						TagID:   tagID,
						RawData: make(map[string]string),
					}

					// Add any additional data from other cells.
					for i := 2; i < len(cells); i++ {
						cell := cells[i]
						fieldValue := ""
						for c := cell.FirstChild; c != nil; c = c.NextSibling {
							if c.Type == html.TextNode {
								fieldValue = strings.TrimSpace(c.Data)
								break
							}
						}

						// Use field index as key if we don't have header info.
						tagRead.RawData[fmt.Sprintf("field_%d", i)] = fieldValue
					}

					tagReads = append(tagReads, tagRead)
				}
			}
		}

		// Process child nodes.
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			processRow(c)
		}
	}

	// Process the table.
	processRow(table)

	return tagReads, nil
}

// extractTotalPages extracts the total number of pages from the response.
func extractTotalPages(htmlBody []byte) int {
	// Look for pagination info like "page X of Y".
	pattern := regexp.MustCompile(`page\s+\d+\s+of\s+(\d+)`)
	matches := pattern.FindSubmatch(htmlBody)

	if len(matches) >= 2 {
		totalPages, err := strconv.Atoi(string(matches[1]))
		if err != nil {
			slog.Error("failed to convert total pages", "value", string(matches[1]), "error", err)
		} else if totalPages > 0 {
			return totalPages
		}
	}

	// Default to 1 if we can't determine total pages.
	return 1
}

// DiscordSender is an interface for sending Discord embed messages.
// It is assumed that the provided bot implements a SendEmbed method.
type DiscordSender interface {
	SendEmbed(embed *discordgo.MessageEmbed)
}

// Start begins a background process that runs every five minutes.
// It fetches the latest tag reads, stores new ones in the database,
// and sends a Discord embed message with a summary of the results.
func (c *Client) Start(discordBot DiscordSender) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			slog.Info("Fetching tag reads for periodic report")
			tagReads, err := c.FetchTagReads()
			if err != nil {
				slog.Error("failed to fetch tag reads", "error", err)
				errorEmbed := &discordgo.MessageEmbed{
					Title:       "Dero ZAP Report Error",
					Description: fmt.Sprintf("Error fetching tag reads: %v", err),
					Color:       0xFF0000, // Red for errors.
					Timestamp:   time.Now().Format(time.RFC3339),
				}
				discordBot.SendEmbed(errorEmbed)
				continue
			}

			// Store new tag reads in the database
			newRecordsCount, err := c.storeNewTagReads(tagReads)
			if err != nil {
				slog.Error("failed to store tag reads", "error", err)
				// Continue with Discord notification even if DB storage failed
			}

			var description string
			if len(tagReads) == 0 {
				description = "No tag reads found in the latest report."
			} else {
				if newRecordsCount > 0 {
					description = fmt.Sprintf("Found %d tag reads (%d new entries added to database):\n",
						len(tagReads), newRecordsCount)
				} else {
					description = fmt.Sprintf("Found %d tag reads (no new entries):\n", len(tagReads))
				}

				// Optionally list the first few tag reads.
				maxItems := 5
				if len(tagReads) < maxItems {
					maxItems = len(tagReads)
				}
				for i := 0; i < maxItems; i++ {
					tr := tagReads[i]
					description += fmt.Sprintf("- Tag %s at %s\n", tr.TagID, tr.Date)
				}
			}

			reportEmbed := &discordgo.MessageEmbed{
				Title:       "Dero ZAP Tag Reads Report",
				Description: description,
				Color:       0x00FF00, // Green for a successful report.
				Timestamp:   time.Now().Format(time.RFC3339),
			}

			discordBot.SendEmbed(reportEmbed)
		}
	}()
}
