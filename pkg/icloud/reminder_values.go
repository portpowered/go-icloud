package icloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	jsonTrueValue                = "true"
	jsonFalseValue               = "false"
	reminderFloatScientificUpper = 1e16
	reminderFloatScientificLower = 1e-4
	reminderBMPMaximum           = '\uffff'
)

func reminderTruthy(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any

	err := decoder.Decode(&value)
	if err != nil {
		return false, fmt.Errorf("decode reminder value: %w", err)
	}

	switch value := value.(type) {
	case nil:
		return false, nil
	case bool:
		return value, nil
	case string:
		return value != "", nil
	case []any:
		return len(value) != 0, nil
	case map[string]any:
		return len(value) != 0, nil
	case json.Number:
		// Compare zero without float conversion, preserving arbitrarily large JSON numbers.
		mantissa, _, _ := strings.Cut(strings.ToLower(value.String()), "e")

		return strings.Trim(mantissa, "-+0.") != "", nil
	default:
		return false, errReminderList
	}
}

func reminderDisplayText(raw json.RawMessage) (string, error) {
	var text string

	err := json.Unmarshal(raw, &text)
	if err == nil && string(raw) != jsonNullValue {
		return text, nil
	}

	return reminderValueRepresentation(bytes.TrimSpace(raw))
}

func reminderValueRepresentation(raw json.RawMessage) (string, error) {
	switch {
	case string(raw) == jsonNullValue:
		return "None", nil
	case string(raw) == jsonTrueValue:
		return "True", nil
	case string(raw) == jsonFalseValue:
		return "False", nil
	case len(raw) == 0:
		return "", errReminderList
	case raw[0] == '"':
		var text string

		err := json.Unmarshal(raw, &text)
		if err != nil {
			return "", fmt.Errorf("decode reminder text value: %w", err)
		}

		return reminderQuotedText(text), nil
	case raw[0] == '[':
		return reminderSequenceRepresentation(raw)
	case raw[0] == '{':
		return reminderObjectRepresentation(raw)
	default:
		return reminderNumericRepresentation(string(raw))
	}
}

func reminderQuotedText(text string) string {
	quote := '\''
	if strings.Contains(text, "'") && !strings.Contains(text, `"`) {
		quote = '"'
	}

	var output strings.Builder

	output.WriteRune(quote)

	for _, character := range text {
		writeReminderRepresentationRune(&output, character, quote)
	}

	output.WriteRune(quote)

	return output.String()
}

func writeReminderRepresentationRune(output *strings.Builder, character, quote rune) {
	switch character {
	case quote, '\\':
		output.WriteRune('\\')
		output.WriteRune(character)
	case '\n':
		output.WriteString(`\n`)
	case '\r':
		output.WriteString(`\r`)
	case '\t':
		output.WriteString(`\t`)
	default:
		writeReminderPrintableRune(output, character)
	}
}

func writeReminderPrintableRune(output *strings.Builder, character rune) {
	if unicode.IsPrint(character) && (!unicode.Is(unicode.Zs, character) || character == ' ') {
		output.WriteRune(character)

		return
	}

	switch {
	case character <= unicode.MaxLatin1:
		fmt.Fprintf(output, `\x%02x`, character)
	case character <= reminderBMPMaximum:
		fmt.Fprintf(output, `\u%04x`, character)
	default:
		fmt.Fprintf(output, `\U%08x`, character)
	}
}

func reminderSequenceRepresentation(raw json.RawMessage) (string, error) {
	var values []json.RawMessage

	err := json.Unmarshal(raw, &values)
	if err != nil {
		return "", fmt.Errorf("decode reminder sequence: %w", err)
	}

	parts := make([]string, 0, len(values))

	for _, value := range values {
		text, valueErr := reminderValueRepresentation(bytes.TrimSpace(value))
		if valueErr != nil {
			return "", valueErr
		}

		parts = append(parts, text)
	}

	return "[" + strings.Join(parts, ", ") + "]", nil
}

func reminderObjectRepresentation(raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))

	_, err := decoder.Token()
	if err != nil {
		return "", fmt.Errorf("decode reminder object: %w", err)
	}

	parts := make([]string, 0)

	for decoder.More() {
		name, nameErr := decoder.Token()
		if nameErr != nil {
			return "", fmt.Errorf("decode reminder object name: %w", nameErr)
		}

		var value json.RawMessage

		err = decoder.Decode(&value)
		if err != nil {
			return "", fmt.Errorf("decode reminder object value: %w", err)
		}

		text, valueErr := reminderValueRepresentation(bytes.TrimSpace(value))
		if valueErr != nil {
			return "", valueErr
		}

		key, valid := name.(string)
		if !valid {
			return "", errReminderList
		}

		parts = append(parts, reminderQuotedText(key)+": "+text)
	}

	return "{" + strings.Join(parts, ", ") + "}", nil
}

func reminderNumericRepresentation(text string) (string, error) {
	if !strings.ContainsAny(text, ".eE") {
		return text, nil
	}

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return "", fmt.Errorf("decode reminder numeric display: %w", err)
	}

	format := byte('f')
	if value >= reminderFloatScientificUpper || value <= -reminderFloatScientificUpper ||
		(value != 0 && value > -reminderFloatScientificLower && value < reminderFloatScientificLower) {
		format = 'e'
	}

	result := strconv.FormatFloat(value, format, -1, 64)
	if !strings.ContainsAny(result, ".e") {
		result += ".0"
	}

	return result, nil
}
