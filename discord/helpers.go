package discord

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// parseDiscordTag parses a struct tag value (e.g. "optional,description:desc,choices:val1|Label1;val2|Label2,default:foo")
// into a map of keys and values.
func parseDiscordTag(tag string) map[string]string {
	parts := strings.Split(tag, ",")
	result := make(map[string]string)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			result[key] = value
		} else {
			result[part] = "true"
		}
	}
	return result
}

// parseChoices parses a choices string (e.g. "val1|Label1;val2|Label2")
// and returns a slice of discordgo.ApplicationCommandOptionChoice.
func parseChoices(s string) []*discordgo.ApplicationCommandOptionChoice {
	var choices []*discordgo.ApplicationCommandOptionChoice
	// Assume choices are separated by semicolons.
	pairs := strings.Split(s, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		// Expect a pair separated by a pipe ("|").
		parts := strings.SplitN(pair, "|", 2)
		var value interface{}
		var name string
		if len(parts) == 2 {
			value = parts[0]
			name = parts[1]
		} else {
			value = parts[0]
			name = parts[0]
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: value,
		})
	}
	return choices
}

// setDefaults iterates over the fields of a struct pointed to by req and, if a field is zero,
// sets it to the default value specified by the "default" key in the "discord" tag.
func setDefaults(req interface{}) error {
	v := reflect.ValueOf(req)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("setDefaults: req is not a pointer to struct")
	}
	v = v.Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := v.Field(i)
		// Only settable fields.
		if !fieldVal.CanSet() {
			continue
		}
		// Check if the field has a zero value.
		if !isZero(fieldVal) {
			continue
		}
		tag := field.Tag.Get("discord")
		if tag == "" {
			continue
		}
		tags := parseDiscordTag(tag)
		if def, ok := tags["default"]; ok && def != "" {
			converted, err := convertType(def, field.Type)
			if err != nil {
				return err
			}
			fieldVal.Set(converted)
		}
	}

	return nil
}

// isZero returns true if v is the zero value for its type.
func isZero(v reflect.Value) bool {
	zero := reflect.Zero(v.Type()).Interface()
	return reflect.DeepEqual(v.Interface(), zero)
}

// convertType converts a string value to a reflect.Value of type t for basic types.
func convertType(val string, t reflect.Type) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf(val), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(i).Convert(t), nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(f).Convert(t), nil
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(b), nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported type for default conversion: %s", t.Kind())
	}
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
