package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/mitchellh/mapstructure"
)

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

// NewBotFunction is a generic constructor that creates a new BotFunctionI command handler.
// It instantiates a GenericBotFunction with a zero-value prototype of type T (your request struct).
// This prototype is later used with the mapstructure decoder to automatically map Discord interaction
// options into your custom request struct. Your request struct can use the "discord" struct tag to control
// how each field is processed. The supported tag options are:
//
//   - optional:   	Marks the field as not required (the command won't error if it's missing).
//   - description: Overrides the auto-generated option description with a custom text.
//   - choices:     Provides a semicolon-separated list of choices in the format "value|Label" for the option.
//   - default:     Specifies a default value to assign if the field remains unset after decoding.
//
// These tags enable you to customize the generated Discord command options and control default values
// and allowed choices via mapstructure.
func NewBotFunction[T Request](name string, handler func(T) (*discordgo.InteractionResponseData, error), autocomplete Autocomplete) BotFunctionI {
	var reqPrototype T
	return &GenericBotFunction[T]{
		Name:             name,
		RequestPrototype: reqPrototype,
		Handler:          handler,
		Autocomplete:     autocomplete,
	}
}
