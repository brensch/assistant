// Package discord provides functionality for handling Discord interactions via Google Cloud Functions
package discord

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Discord interaction types
const (
	InteractionTypePing               = 1
	InteractionTypeApplicationCommand = 2
	InteractionTypeMessageComponent   = 3
	InteractionTypeAutocomplete       = 4
	InteractionTypeModalSubmit        = 5
)

// Discord interaction response types
const (
	ResponseTypePong                             = 1
	ResponseTypeChannelMessageWithSource         = 4
	ResponseTypeDeferredChannelMessageWithSource = 5
	ResponseTypeDeferredUpdateMessage            = 6
	ResponseTypeUpdateMessage                    = 7
	ResponseTypeAutocompleteResult               = 8
	ResponseTypeModal                            = 9
)

// Discord message flags
const (
	FlagEphemeral = 1 << 6 // Makes response only visible to the sender
)

// Command option types
const (
	OptionTypeSubCommand      = 1
	OptionTypeSubCommandGroup = 2
	OptionTypeString          = 3
	OptionTypeInteger         = 4
	OptionTypeBoolean         = 5
	OptionTypeUser            = 6
	OptionTypeChannel         = 7
	OptionTypeRole            = 8
	OptionTypeMentionable     = 9
	OptionTypeNumber          = 10
	OptionTypeAttachment      = 11
)

// Interaction represents a Discord interaction
type Interaction struct {
	ID            string          `json:"id"`
	ApplicationID string          `json:"application_id"`
	Type          int             `json:"type"`
	Data          json.RawMessage `json:"data"` // Data structure varies based on interaction type
	GuildID       string          `json:"guild_id,omitempty"`
	ChannelID     string          `json:"channel_id,omitempty"`
	Member        json.RawMessage `json:"member,omitempty"`
	User          json.RawMessage `json:"user,omitempty"`
	Token         string          `json:"token"`
	Version       int             `json:"version"`
	Message       json.RawMessage `json:"message,omitempty"`
}

// ApplicationCommandData represents the data for an application command interaction
type ApplicationCommandData struct {
	ID      string                     `json:"id"`
	Name    string                     `json:"name"`
	Type    int                        `json:"type"`
	Options []ApplicationCommandOption `json:"options,omitempty"`
}

// ApplicationCommandOption represents an option for an application command
type ApplicationCommandOption struct {
	Name    string                     `json:"name"`
	Type    int                        `json:"type"`
	Value   interface{}                `json:"value,omitempty"`
	Options []ApplicationCommandOption `json:"options,omitempty"`
	Focused bool                       `json:"focused,omitempty"` // Used for autocomplete
}

// InteractionResponse represents a response to a Discord interaction
type InteractionResponse struct {
	Type int                     `json:"type"`
	Data InteractionResponseData `json:"data,omitempty"`
}

// InteractionResponseData represents the data for an interaction response
type InteractionResponseData struct {
	TTS             bool             `json:"tts,omitempty"`
	Content         string           `json:"content,omitempty"`
	Embeds          []Embed          `json:"embeds,omitempty"`
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"`
	Flags           int              `json:"flags,omitempty"`
	Components      []Component      `json:"components,omitempty"`
}

// Embed represents a Discord embed
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Color       int          `json:"color,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Image       *EmbedImage  `json:"image,omitempty"`
	Thumbnail   *EmbedImage  `json:"thumbnail,omitempty"`
	Author      *EmbedAuthor `json:"author,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
}

// EmbedFooter represents the footer of a Discord embed
type EmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedImage represents an image in a Discord embed
type EmbedImage struct {
	URL string `json:"url"`
}

// EmbedAuthor represents the author of a Discord embed
type EmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedField represents a field in a Discord embed
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// AllowedMentions controls which mentions are allowed in a message
type AllowedMentions struct {
	Parse       []string `json:"parse,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Users       []string `json:"users,omitempty"`
	RepliedUser bool     `json:"replied_user,omitempty"`
}

// Component represents a Discord message component
type Component struct {
	Type        int         `json:"type"`
	CustomID    string      `json:"custom_id,omitempty"`
	Label       string      `json:"label,omitempty"`
	Style       int         `json:"style,omitempty"`
	Emoji       *Emoji      `json:"emoji,omitempty"`
	URL         string      `json:"url,omitempty"`
	Disabled    bool        `json:"disabled,omitempty"`
	Components  []Component `json:"components,omitempty"`
	Options     []Option    `json:"options,omitempty"`
	Placeholder string      `json:"placeholder,omitempty"`
	MinValues   *int        `json:"min_values,omitempty"`
	MaxValues   *int        `json:"max_values,omitempty"`
}

// Emoji represents a Discord emoji
type Emoji struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// Option represents a select menu option
type Option struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Emoji       *Emoji `json:"emoji,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// CommandHandler is a function type that handles Discord commands
type CommandHandler func(interaction *Interaction, options []ApplicationCommandOption) (*InteractionResponse, error)

// Command defines a Discord slash command
type Command struct {
	Name        string
	Description string
	Options     []CommandOption
	Handler     CommandHandler
}

// CommandOption defines an option for a Discord slash command
type CommandOption struct {
	Name        string
	Description string
	Type        int
	Required    bool
	Choices     []CommandChoice
	Options     []CommandOption // For subcommands
}

// CommandChoice defines a choice for a command option
type CommandChoice struct {
	Name  string
	Value interface{}
}

// CommandRegistry stores and manages available commands
type CommandRegistry struct {
	commands map[string]*Command
	mu       sync.RWMutex
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*Command),
	}
}

// RegisterCommand adds a command to the registry
func (r *CommandRegistry) RegisterCommand(cmd *Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[cmd.Name] = cmd
}

// GetCommand retrieves a command by name
func (r *CommandRegistry) GetCommand(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

// ListCommands returns all registered commands
func (r *CommandRegistry) ListCommands() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmds := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// Global registry for commands
var Registry = NewCommandRegistry()

// DiscordHandler is the main HTTP handler for Discord interactions
func DiscordHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Println("got request")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// In production, verify the request signature
	if os.Getenv("ENVIRONMENT") != "development" {
		if !verifySignature(r) {
			http.Error(w, "Invalid request signature", http.StatusUnauthorized)
			return
		}
	}

	// Read the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	// Replace the request body for later use
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// Parse the interaction
	var interaction Interaction
	if err := json.Unmarshal(body, &interaction); err != nil {
		log.Printf("Error parsing interaction: %v", err)
		http.Error(w, "Invalid interaction format", http.StatusBadRequest)
		return
	}

	// Handle different interaction types
	var response *InteractionResponse

	switch interaction.Type {
	case InteractionTypePing:
		// Respond to ping with pong (required for Discord verification)
		response = &InteractionResponse{
			Type: ResponseTypePong,
		}
	case InteractionTypeApplicationCommand:
		// Handle application command
		response = handleCommand(&interaction)
	default:
		// Unsupported interaction type
		log.Printf("Unsupported interaction type: %d", interaction.Type)
		response = &InteractionResponse{
			Type: ResponseTypeChannelMessageWithSource,
			Data: InteractionResponseData{
				Content: "This interaction type is not supported yet.",
				Flags:   FlagEphemeral, // Only visible to the user who triggered it
			},
		}
	}

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// verifySignature verifies the signature of a Discord interaction request
func verifySignature(r *http.Request) bool {
	// Get the public key from environment variables
	// publicKey := os.Getenv("DISCORD_PUBLIC_KEY")
	// if publicKey == "" {
	// 	log.Println("DISCORD_PUBLIC_KEY environment variable not set")
	// 	return false
	// }
	publicKey := pubKey

	// Get the signature and timestamp from headers
	signature := r.Header.Get("X-Signature-Ed25519")
	timestamp := r.Header.Get("X-Signature-Timestamp")

	if signature == "" || timestamp == "" {
		log.Println("Missing signature or timestamp headers")
		return false
	}

	// Read the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return false
	}
	// Replace the request body for later use
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// Decode the signature
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		log.Printf("Error decoding signature: %v", err)
		return false
	}

	// Decode the public key
	pubKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		log.Printf("Error decoding public key: %v", err)
		return false
	}

	// Verify the signature
	message := []byte(timestamp + string(body))
	return ed25519.Verify(pubKeyBytes, message, signatureBytes)
}

// handleCommand processes command interactions
func handleCommand(interaction *Interaction) *InteractionResponse {
	// Parse the command data
	var cmdData ApplicationCommandData
	if err := json.Unmarshal(interaction.Data, &cmdData); err != nil {
		log.Printf("Error parsing command data: %v", err)
		return errorResponse("Failed to parse command data")
	}

	// Find the command in the registry
	cmd, ok := Registry.GetCommand(cmdData.Name)
	if !ok {
		log.Printf("Unknown command: %s", cmdData.Name)
		return errorResponse(fmt.Sprintf("Unknown command: %s", cmdData.Name))
	}

	// Execute the command handler
	response, err := cmd.Handler(interaction, cmdData.Options)
	if err != nil {
		log.Printf("Error handling command %s: %v", cmdData.Name, err)
		return errorResponse(fmt.Sprintf("Error executing command: %v", err))
	}

	return response
}

// errorResponse creates a simple error response
func errorResponse(message string) *InteractionResponse {
	return &InteractionResponse{
		Type: ResponseTypeChannelMessageWithSource,
		Data: InteractionResponseData{
			Content: message,
			Flags:   FlagEphemeral, // Only visible to the user who triggered it
		},
	}
}

// GetOptionValue retrieves an option value by name
func GetOptionValue(options []ApplicationCommandOption, name string) (interface{}, bool) {
	for _, opt := range options {
		if opt.Name == name {
			return opt.Value, true
		}

		// Check in sub-options if present
		if len(opt.Options) > 0 {
			if val, found := GetOptionValue(opt.Options, name); found {
				return val, true
			}
		}
	}
	return nil, false
}

// RegisterCommand adds a command to the global registry
func RegisterCommand(cmd *Command) {
	Registry.RegisterCommand(cmd)
}

// SendFollowupMessage sends a followup message after an interaction
func SendFollowupMessage(applicationID, interactionToken string, message InteractionResponseData) error {
	url := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s", applicationID, interactionToken)
	return sendWebhookRequest("POST", url, message)
}

// EditOriginalResponse edits the original interaction response
func EditOriginalResponse(applicationID, interactionToken string, message InteractionResponseData) error {
	url := fmt.Sprintf("https://discord.com/api/v10/webhooks/%s/%s/messages/@original", applicationID, interactionToken)
	return sendWebhookRequest("PATCH", url, message)
}

// sendWebhookRequest sends a request to Discord's webhook API
func sendWebhookRequest(method, url string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook data: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CommandBuilder provides a fluent API for building commands
type CommandBuilder struct {
	cmd *Command
}

// NewCommand creates a new command builder
func NewCommand(name, description string) *CommandBuilder {
	return &CommandBuilder{
		cmd: &Command{
			Name:        name,
			Description: description,
			Options:     []CommandOption{},
		},
	}
}

// AddOption adds an option to the command
func (b *CommandBuilder) AddOption(name, description string, optType int, required bool) *CommandBuilder {
	b.cmd.Options = append(b.cmd.Options, CommandOption{
		Name:        name,
		Description: description,
		Type:        optType,
		Required:    required,
	})
	return b
}

// AddStringOption adds a string option to the command
func (b *CommandBuilder) AddStringOption(name, description string, required bool) *CommandBuilder {
	return b.AddOption(name, description, OptionTypeString, required)
}

// AddIntegerOption adds an integer option to the command
func (b *CommandBuilder) AddIntegerOption(name, description string, required bool) *CommandBuilder {
	return b.AddOption(name, description, OptionTypeInteger, required)
}

// AddBooleanOption adds a boolean option to the command
func (b *CommandBuilder) AddBooleanOption(name, description string, required bool) *CommandBuilder {
	return b.AddOption(name, description, OptionTypeBoolean, required)
}

// AddUserOption adds a user option to the command
func (b *CommandBuilder) AddUserOption(name, description string, required bool) *CommandBuilder {
	return b.AddOption(name, description, OptionTypeUser, required)
}

// WithHandler sets the command handler
func (b *CommandBuilder) WithHandler(handler CommandHandler) *CommandBuilder {
	b.cmd.Handler = handler
	return b
}

// Register registers the command with the global registry
func (b *CommandBuilder) Register() *Command {
	RegisterCommand(b.cmd)
	return b.cmd
}

// Initialize default commands
func init() {
	// Register a ping command as an example
	NewCommand("ping", "Check if the bot is responding").
		WithHandler(func(interaction *Interaction, options []ApplicationCommandOption) (*InteractionResponse, error) {
			return &InteractionResponse{
				Type: ResponseTypeChannelMessageWithSource,
				Data: InteractionResponseData{
					Content: "Pong! Bot is up and running.",
				},
			}, nil
		}).
		Register()

	// Register a help command to show available commands
	NewCommand("help", "Display available commands").
		WithHandler(func(interaction *Interaction, options []ApplicationCommandOption) (*InteractionResponse, error) {
			commands := Registry.ListCommands()

			// Create an embed with all commands
			fields := make([]EmbedField, 0, len(commands))
			for _, cmd := range commands {
				fields = append(fields, EmbedField{
					Name:   "/" + cmd.Name,
					Value:  cmd.Description,
					Inline: false,
				})
			}

			return &InteractionResponse{
				Type: ResponseTypeChannelMessageWithSource,
				Data: InteractionResponseData{
					Embeds: []Embed{
						{
							Title:       "Available Commands",
							Description: fmt.Sprintf("Here are the %d available commands:", len(commands)),
							Color:       0x3498db, // Blue color
							Fields:      fields,
						},
					},
				},
			}, nil
		}).
		Register()
}
