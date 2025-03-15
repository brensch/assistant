package discord

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// RegisterCustomCommands adds custom commands to the bot
func RegisterCustomCommands() {
	// Register a 'roll' command for rolling dice
	NewCommand("roll", "Roll a dice").
		AddStringOption("dice", "Dice notation (e.g., 2d6, 1d20)", true).
		WithHandler(func(interaction *Interaction, options []ApplicationCommandOption) (*InteractionResponse, error) {
			// Extract the dice notation
			diceNotation, found := GetOptionValue(options, "dice")
			if !found {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Please provide a dice notation (e.g., 2d6, 1d20).",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			diceStr, ok := diceNotation.(string)
			if !ok {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Invalid dice notation format.",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			// Parse the dice notation (format: NdM, where N is the number of dice and M is the number of sides)
			result, rolls, err := rollDice(diceStr)
			if err != nil {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: fmt.Sprintf("Error: %v", err),
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			return &InteractionResponse{
				Type: ResponseTypeChannelMessageWithSource,
				Data: InteractionResponseData{
					Content: fmt.Sprintf("🎲 **Dice Roll**: %s\n**Result**: %d\n**Individual Rolls**: %v", diceStr, result, rolls),
				},
			}, nil
		}).
		Register()

	// Register a 'quote' command to provide random quotes
	NewCommand("quote", "Get a random inspirational quote").
		WithHandler(func(interaction *Interaction, options []ApplicationCommandOption) (*InteractionResponse, error) {
			quotes := []string{
				"The only way to do great work is to love what you do. - Steve Jobs",
				"Life is what happens when you're busy making other plans. - John Lennon",
				"The future belongs to those who believe in the beauty of their dreams. - Eleanor Roosevelt",
				"In the end, it's not the years in your life that count. It's the life in your years. - Abraham Lincoln",
				"The purpose of our lives is to be happy. - Dalai Lama",
			}

			rand.Seed(time.Now().UnixNano())
			randomQuote := quotes[rand.Intn(len(quotes))]

			return &InteractionResponse{
				Type: ResponseTypeChannelMessageWithSource,
				Data: InteractionResponseData{
					Embeds: []Embed{
						{
							Title:       "Inspirational Quote",
							Description: randomQuote,
							Color:       0x4CAF50, // Green color
						},
					},
				},
			}, nil
		}).
		Register()

	// Register a 'poll' command to create polls
	NewCommand("poll", "Create a poll").
		AddStringOption("question", "The poll question", true).
		AddStringOption("options", "Options separated by commas", true).
		WithHandler(func(interaction *Interaction, options []ApplicationCommandOption) (*InteractionResponse, error) {
			// Extract the question
			questionValue, found := GetOptionValue(options, "question")
			if !found {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Please provide a poll question.",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			question, ok := questionValue.(string)
			if !ok {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Invalid question format.",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			// Extract the options
			optionsValue, found := GetOptionValue(options, "options")
			if !found {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Please provide poll options.",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			optionsStr, ok := optionsValue.(string)
			if !ok {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Invalid options format.",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			// Split options by commas
			optionsList := strings.Split(optionsStr, ",")
			if len(optionsList) < 2 || len(optionsList) > 10 {
				return &InteractionResponse{
					Type: ResponseTypeChannelMessageWithSource,
					Data: InteractionResponseData{
						Content: "Please provide between 2 and 10 options.",
						Flags:   FlagEphemeral,
					},
				}, nil
			}

			// Create an embed for the poll
			description := fmt.Sprintf("**%s**\n\n", question)

			// Unicode emojis for options (numbers 1-10)
			emojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

			for i, option := range optionsList {
				if i < len(emojis) {
					description += fmt.Sprintf("%s %s\n", emojis[i], strings.TrimSpace(option))
				}
			}

			description += "\nReact with the emoji corresponding to your choice!"

			return &InteractionResponse{
				Type: ResponseTypeChannelMessageWithSource,
				Data: InteractionResponseData{
					Embeds: []Embed{
						{
							Title:       "Poll",
							Description: description,
							Color:       0xE91E63, // Pink color
						},
					},
				},
			}, nil
		}).
		Register()
}

// rollDice parses a dice notation string and returns the result
func rollDice(notation string) (total int, rolls []int, err error) {
	// Parse the dice notation (format: NdM)
	var numDice, numSides int
	_, err = fmt.Sscanf(notation, "%dd%d", &numDice, &numSides)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid dice notation: use format NdM (e.g., 2d6)")
	}

	// Validate input
	if numDice <= 0 || numDice > 100 {
		return 0, nil, fmt.Errorf("number of dice must be between 1 and 100")
	}
	if numSides <= 0 || numSides > 1000 {
		return 0, nil, fmt.Errorf("number of sides must be between 1 and 1000")
	}

	// Roll the dice
	rand.Seed(time.Now().UnixNano())
	rolls = make([]int, numDice)
	total = 0

	for i := 0; i < numDice; i++ {
		roll := rand.Intn(numSides) + 1
		rolls[i] = roll
		total += roll
	}

	return total, rolls, nil
}
