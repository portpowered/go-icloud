package webtransport

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/remindersdate"
)

func reminderIntegerJSON(raw json.RawMessage) (json.RawMessage, error) {
	if !reminderSyncInteger(raw) {
		return nil, errReminderZonesShape
	}

	text := reminderNumberText(raw)

	value, valid := new(big.Rat).SetString(text)
	if !valid || !value.IsInt() {
		return nil, errReminderZonesShape
	}

	return json.RawMessage(value.Num().String()), nil
}

func reminderNumberText(raw json.RawMessage) string {
	if string(raw) == jsonTrueValue {
		return "1"
	}

	if string(raw) == jsonFalseValue {
		return "0"
	}

	var text string
	if json.Unmarshal(raw, &text) != nil {
		text = string(raw)
	}

	return strings.ReplaceAll(strings.TrimSpace(text), "_", "")
}

func normalizeReminderAudit(fields map[string]json.RawMessage, name string) error {
	raw, supplied := fields[name]
	if !supplied || string(raw) == jsonNullValue {
		return nil
	}

	audit, err := accountFields(raw)
	if err != nil {
		return err
	}

	timestamp := audit[protocol.RemindersCKAuditInfoTimestamp]

	value, err := reminderAuditMillis(timestamp)
	if err != nil {
		return err
	}

	audit[protocol.RemindersCKAuditInfoTimestamp] = json.RawMessage(strconv.FormatInt(value, 10))

	fields[name], err = json.Marshal(audit)
	if err != nil {
		return fmt.Errorf("encode reminder audit: %w", err)
	}

	return nil
}

func reminderAuditMillis(raw json.RawMessage) (int64, error) {
	text := reminderNumberText(raw)

	var input string
	if json.Unmarshal(raw, &input) == nil {
		validText, err := regexp.MatchString(protocol.RemindersCKAuditTimestampInputTextPattern, input)
		if err != nil || !validText {
			return 0, errReminderZonesShape
		}
	}

	millis, err := reminderAuditInteger(text, json.Unmarshal(raw, &input) == nil)
	if err != nil {
		return 0, err
	}

	if remindersdate.FromMillis(millis) == nil {
		return 0, errReminderZonesShape
	}

	return millis, nil
}

// Source JSON fractional/exponent numbers are float64 before int(v). Text and
// integer JSON inputs retain integer semantics, including at date boundaries.
func reminderAuditInteger(text string, quoted bool) (int64, error) {
	if !quoted && strings.ContainsAny(text, ".eE") {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, fmt.Errorf("decode reminder audit number: %w", err)
		}

		return int64(value), nil
	}

	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode reminder audit integer: %w", err)
	}

	return value, nil
}
