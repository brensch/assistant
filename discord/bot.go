package discord

import (
	"fmt"
	"reflect"
	"strings"

	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/mitchellh/mapstructure"
)

// Bot encapsulates the discordgo session, configuration, and registered functions.
type Bot struct {
	session   *discordgo.Session
	config    BotConfig
	functions []BotFunctionI
}

// Request is a blank interface for the command request definitions.
type Request interface{}

// Autocomplete is an interface for types that can provide autocomplete suggestions.
type Autocomplete interface {
	// Complete takes an input string and returns a list of choices for the option.
	Complete(input string) ([]*discordgo.ApplicationCommandOptionChoice, error)
}

// BotFunctionI is the common interface for all bot command functions.
type BotFunctionI interface {
	GetName() string
	GetRequestPrototype() Request
	// HandleInteraction decodes interaction data into a request struct and calls the handler.
	// It returns the response data that can be sent directly to Discord.
	HandleInteraction(data *discordgo.ApplicationCommandInteractionData) (*discordgo.InteractionResponseData, error)
}

// GenericBotFunction is a generic implementation of BotFunctionI.
type GenericBotFunction[T Request] struct {
	// Name is the command name.
	Name string
	// RequestPrototype is an instance of the request type (typically the zero value)
	// used for reflection to generate command options.
	RequestPrototype T
	// Handler is the function to execute for the command.
	Handler func(T) (*discordgo.InteractionResponseData, error)
	// Autocomplete is an optional implementation for providing autocomplete choices.
	Autocomplete Autocomplete
}

// GetName returns the command's name.
func (bf *GenericBotFunction[T]) GetName() string {
	return bf.Name
}

// GetRequestPrototype returns the command's request prototype.
func (bf *GenericBotFunction[T]) GetRequestPrototype() Request {
	return bf.RequestPrototype
}

// HandleInteraction processes the interaction by constructing a request of type T from the data
// and then invoking the handler. It decodes the options using mapstructure and then applies any defaults.
func (bf *GenericBotFunction[T]) HandleInteraction(data *discordgo.ApplicationCommandInteractionData) (*discordgo.InteractionResponseData, error) {
	var req T

	// Build a map from option name to its value.
	optsMap := make(map[string]interface{})
	for _, opt := range data.Options {
		optsMap[opt.Name] = opt.Value
	}

	// Decode into req using mapstructure with our custom tag.
	decoderConfig := mapstructure.DecoderConfig{
		TagName:          "discord",
		Result:           &req,
		WeaklyTypedInput: true, // helps convert numbers and booleans automatically.
	}
	decoder, err := mapstructure.NewDecoder(&decoderConfig)
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(optsMap); err != nil {
		return nil, err
	}

	// Set default values on fields that are still zero.
	if err := setDefaults(&req); err != nil {
		return nil, err
	}

	return bf.Handler(req)
}

// NewBotFunction is a generic constructor that returns a BotFunctionI.
func NewBotFunction[T Request](name string, handler func(T) (*discordgo.InteractionResponseData, error), autocomplete Autocomplete) BotFunctionI {
	var reqPrototype T
	return &GenericBotFunction[T]{
		Name:             name,
		RequestPrototype: reqPrototype,
		Handler:          handler,
		Autocomplete:     autocomplete,
	}
}

// BotConfig contains configuration for the bot.
type BotConfig struct {
	AppID    string
	BotToken string
}

// structToCommandOptions uses reflection to generate Discord command options from a request struct.
// It also uses custom struct tags (key "discord") for options like optional, choices, description, and default.
func structToCommandOptions(req Request) ([]*discordgo.ApplicationCommandOption, error) {
	t := reflect.TypeOf(req)
	// If req is a pointer, get the underlying value and type.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("request is not a struct")
	}

	var options []*discordgo.ApplicationCommandOption
	// Iterate over struct fields.
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		optionName := strings.ToLower(field.Name)
		var optionType discordgo.ApplicationCommandOptionType

		// Map common Go types to Discord option types.
		switch field.Type.Kind() {
		case reflect.String:
			optionType = discordgo.ApplicationCommandOptionString
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			optionType = discordgo.ApplicationCommandOptionInteger
		case reflect.Float32, reflect.Float64:
			optionType = discordgo.ApplicationCommandOptionNumber
		case reflect.Bool:
			optionType = discordgo.ApplicationCommandOptionBoolean
		default:
			optionType = discordgo.ApplicationCommandOptionString
		}

		// Defaults.
		required := true
		description := "Auto-generated option for " + optionName
		var choices []*discordgo.ApplicationCommandOptionChoice

		// Parse custom struct tag if present.
		if tagValue := field.Tag.Get("discord"); tagValue != "" {
			tags := parseDiscordTag(tagValue)
			if _, ok := tags["optional"]; ok {
				required = false
			}
			if desc, ok := tags["description"]; ok && desc != "" {
				description = desc
			}
			if choicesStr, ok := tags["choices"]; ok && choicesStr != "" {
				choices = parseChoices(choicesStr)
			}
		}

		opt := &discordgo.ApplicationCommandOption{
			Type:        optionType,
			Name:        optionName,
			Description: description,
			Required:    required,
			Choices:     choices,
		}
		options = append(options, opt)
	}

	return options, nil
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
