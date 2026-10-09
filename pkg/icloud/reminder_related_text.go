package icloud

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"golang.org/x/text/encoding/unicode"
)

func reminderRelatedText(record cloudkit.CKRecord, name string, replaceInvalid bool) (json.RawMessage, error) {
	raw, err := reminderField(record, name)
	if err != nil || record.Fields == nil {
		return raw, err
	}

	field, exists := (*record.Fields)[name]
	if !exists {
		return raw, nil
	}

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return nil, fmt.Errorf("decode related text wrapper: %w", err)
	}

	if wrapper.Type != string(cloudkit.BYTES) && wrapper.Type != string(cloudkit.ENCRYPTEDBYTES) {
		return raw, nil
	}

	return reminderRelatedByteText(raw, replaceInvalid)
}

func reminderRelatedByteText(raw json.RawMessage, replaceInvalid bool) (json.RawMessage, error) {
	var encoded string

	err := json.Unmarshal(raw, &encoded)
	if err != nil {
		return nil, fmt.Errorf("decode related text input: %w", err)
	}

	decoded, err := reminderRelatedBase64(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode related text bytes: %w", err)
	}

	if !replaceInvalid && !utf8.Valid(decoded) {
		return nil, errReminderList
	}

	text, err := unicode.UTF8.NewDecoder().String(string(decoded))
	if err != nil {
		return nil, fmt.Errorf("decode related UTF-8 text: %w", err)
	}

	value, err := json.Marshal(text)
	if err != nil {
		return nil, fmt.Errorf("encode related text: %w", err)
	}

	return value, nil
}
