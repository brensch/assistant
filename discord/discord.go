package discord

import (
	"encoding/json"
	"log"
	"net/http"

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
		log.Printf("Cannot create slash command 'sayhi': %v", err)
	}

	// List all guilds (builds) and send a greeting message to each guild’s system channel (if available).
	guilds := dg.State.Guilds
	for _, g := range guilds {
		log.Printf("Bot is in guild: %s (%s)", g.Name, g.ID)
		if g.SystemChannelID != "" {
			_, err = dg.ChannelMessageSend(g.SystemChannelID, "Bot is online! Use /sayhi to interact with me.")
			if err != nil {
				log.Printf("Failed to send greeting to guild %s: %v", g.ID, err)
			}
		} else {
			log.Printf("Guild %s does not have a system channel set.", g.ID)
		}
	}

	return bot, nil
}

// onMessageCreate logs every message the bot sees (ignoring its own).
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	log.Printf("Message from %s in channel %s: %s", m.Author.Username, m.ChannelID, m.Content)
}

// onInteractionCreate logs each interaction and responds to the "sayhi" command.
func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var username string
	if i.Member != nil && i.Member.User != nil {
		username = i.Member.User.Username
	} else if i.User != nil {
		username = i.User.Username
	} else {
		username = "unknown"
	}

	// Use ApplicationCommandData() to obtain command-specific data.
	cmdData := i.ApplicationCommandData()
	log.Printf("Interaction from %s: command %s", username, cmdData.Name)

	if cmdData.Name == "sayhi" {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Hi there!",
			},
		})
		if err != nil {
			log.Printf("Failed to respond to sayhi interaction: %v", err)
		}
	}
}

// Handler serves as the HTTP endpoint for Discord interactions.
func (b *Bot) Handler(w http.ResponseWriter, r *http.Request) {
	var interaction discordgo.Interaction
	if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Use ApplicationCommandData() to get the command name.
	cmdData := interaction.ApplicationCommandData()
	log.Printf("HTTP Interaction received: command %s", cmdData.Name)

	// Handle Ping requests.
	if interaction.Type == discordgo.InteractionPing {
		// In discordgo v0.28.1, respond with type 1 (Pong).
		response := discordgo.InteractionResponse{
			Type: 1, // Pong response.
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle application commands.
	if interaction.Type == discordgo.InteractionApplicationCommand {
		if cmdData.Name == "sayhi" {
			response := discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Hi there!",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// For any unhandled interactions, return a not-implemented error.
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}
