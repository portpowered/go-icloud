package webtransport

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
)

const (
	jsonTrueValue  = "true"
	jsonFalseValue = "false"
)

func reminderSyncInteger(raw json.RawMessage) bool {
	if string(raw) == jsonTrueValue || string(raw) == jsonFalseValue {
		return true
	}

	text := string(raw)

	var decoded string
	if json.Unmarshal(raw, &decoded) == nil {
		text = strings.TrimSpace(decoded)

		valid, err := regexp.MatchString(protocol.RemindersCKIntegerInputTextPattern, text)
		if err != nil || !valid {
			return false
		}

		text = strings.ReplaceAll(text, "_", "")
	}

	if decoded == "" && strings.ContainsAny(text, ".eE") {
		return reminderSyncFloatInteger(text)
	}

	value, valid := new(big.Rat).SetString(text)

	return valid && value.IsInt()
}

func reminderSyncFloatInteger(text string) bool {
	number, err := strconv.ParseFloat(text, 64)

	return (err == nil || errors.Is(err, strconv.ErrRange)) &&
		number > math.MinInt64 && number < float64(math.MaxInt64) && math.Trunc(number) == number
}

func reminderSyncDouble(raw json.RawMessage) bool {
	if string(raw) == jsonTrueValue || string(raw) == jsonFalseValue {
		return true
	}

	text := string(raw)

	var decoded string
	if json.Unmarshal(raw, &decoded) == nil {
		text = strings.TrimSpace(decoded)
	}

	_, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)

	return err == nil
}

func reminderSyncTimestamp(raw json.RawMessage) bool {
	if string(raw) == jsonNullValue || string(raw) == jsonTrueValue || string(raw) == jsonFalseValue {
		return true
	}

	var text string
	if json.Unmarshal(raw, &text) == nil {
		return true
	}

	var number json.Number

	return json.Unmarshal(raw, &number) == nil
}

func reminderSyncBytes(raw json.RawMessage) bool {
	var text string
	if string(raw) == jsonNullValue || json.Unmarshal(raw, &text) != nil {
		return false
	}

	valid, err := regexp.MatchString(protocol.RemindersCKBase64InputPattern, text)

	return err == nil && valid
}

func reminderSyncBoolean(raw json.RawMessage) bool {
	if string(raw) == jsonNullValue || string(raw) == jsonTrueValue || string(raw) == jsonFalseValue {
		return true
	}

	var text string
	if json.Unmarshal(raw, &text) == nil {
		valid, err := regexp.MatchString(protocol.RemindersCKBooleanInputTextPattern, text)

		return err == nil && valid
	}

	var number float64

	return json.Unmarshal(raw, &number) == nil && (number == 0 || number == 1)
}
