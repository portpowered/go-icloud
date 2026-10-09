package icloud

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"golang.org/x/text/encoding/unicode"
)

func reminderRelatedBytes(record cloudkit.CKRecord, name string) (bool, error) {
	if record.Fields == nil {
		return false, nil
	}

	field, exists := (*record.Fields)[name]
	if !exists {
		return false, nil
	}

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return false, fmt.Errorf("decode related field wrapper: %w", err)
	}

	return wrapper.Type == string(cloudkit.BYTES) || wrapper.Type == string(cloudkit.ENCRYPTEDBYTES), nil
}

func reminderRelatedKind(record cloudkit.CKRecord) (string, error) {
	bytes, err := reminderRelatedBytes(record, protocol.RemindersRelatedFieldTypeValue)
	if err != nil || bytes {
		return "", err
	}

	raw, err := reminderField(record, protocol.RemindersRelatedFieldTypeValue)
	if err != nil {
		return "", err
	}

	// Source compares the uncoerced field to a string before constructing its model.
	if len(raw) == 0 || raw[0] != '"' {
		return "", nil
	}

	var kind string

	err = json.Unmarshal(raw, &kind)
	if err != nil {
		return "", fmt.Errorf("decode related record type: %w", err)
	}

	return kind, nil
}

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
