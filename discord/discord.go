// Package discord implements a Discord bot with methods for handling interactions
// and sending messages. It registers a global slash command ("say hi") on initialization.
package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Constants used in Discord interactions.
const (
	// Incoming interaction types.
	PingInteraction    = 1
	ApplicationCommand = 2
	// Outgoing response types.
	PongResponse             = 1
	ChannelMessageWithSource = 4
)

// Interaction represents a minimal Discord interaction payload.
type Interaction struct {
	Type  int              `json:"type"`
	Data  *InteractionData `json:"data,omitempty"`
	Token string           `json:"token"`
	ID    string           `json:"id"`
}

// InteractionData holds data for application commands.
type InteractionData struct {
	Name string `json:"name"`
}

// InteractionResponse is used to respond to an interaction.
type InteractionResponse struct {
	Type int                      `json:"type"`
	Data *InteractionCallbackData `json:"data,omitempty"`
}

// InteractionCallbackData holds the message to be sent back.
type InteractionCallbackData struct {
	Content string `json:"content"`
}

// CommandPayload defines the JSON structure for a slash command.
type CommandPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        int    `json:"type"` // Add this field
}

// Bot holds configuration for your Discord bot.
type Bot struct {
	AppID      string
	BotToken   string
	ChannelIDs []string
}

// BotConfig holds parameters used to initialize a Bot.
type BotConfig struct {
	AppID      string
	BotToken   string
	ChannelIDs []string
}

// NewBot creates a new Bot instance. It registers a global slash command ("say hi")
// so that Discord knows to send interactions to your endpoint.
func NewBot(cfg BotConfig) (*Bot, error) {
	bot := &Bot{
		AppID:      cfg.AppID,
		BotToken:   cfg.BotToken,
		ChannelIDs: cfg.ChannelIDs,
	}

	// Register the global command "say hi".
	err := bot.registerGlobalCommand(CommandPayload{
		Name:        "sayhi", // no spaces, all lowercase
		Description: "Replies with hi",
		Type:        1, // CHAT_INPUT command
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register global command: %w", err)
	}
	return bot, nil
}

// registerGlobalCommand registers a global slash command with Discord.
func (b *Bot) registerGlobalCommand(payload CommandPayload) error {
	url := fmt.Sprintf("https://discord.com/api/v10/applications/%s/commands", b.AppID)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+b.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to register command, status: %s", resp.Status)
	}
	return nil
}

// Handler is an HTTP handler method for processing Discord interactions.
// For the slash command "say hi", it replies with the text "hi".
func (b *Bot) Handler(w http.ResponseWriter, r *http.Request) {
	// Read the request body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the JSON payload.
	var interaction Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Respond to Discord's PING.
	if interaction.Type == PingInteraction {
		resp := InteractionResponse{Type: PongResponse}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Process application commands.
	if interaction.Type == ApplicationCommand && interaction.Data != nil {
		if interaction.Data.Name == "say hi" {
			resp := InteractionResponse{
				Type: ChannelMessageWithSource,
				Data: &InteractionCallbackData{
					Content: "hi",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}

	// For any other interactions, simply return 200 OK.
	w.WriteHeader(http.StatusOK)
}

// SendMessage sends the given message to every channel listed in the Bot's ChannelIDs.
// It makes an HTTP POST to Discord's channel messages endpoint for each channel.
func (b *Bot) SendMessage(message string) error {
	for _, channelID := range b.ChannelIDs {
		url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
		payload := map[string]string{
			"content": message,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bot "+b.BotToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error sending message to channel %s: %v", channelID, err)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("Non-2xx response sending message to channel %s: %s", channelID, resp.Status)
		}
		_ = resp.Body.Close()
	}
	return nil
}
