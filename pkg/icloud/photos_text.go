package icloud

import (
	"encoding/json"
	"fmt"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"unicode"
	"unicode/utf8"
)

func photoEncryptedText(raw json.RawMessage) (string, bool) {
	var value string
	if len(raw) == 0 || string(raw) == jsonNullValue || json.Unmarshal(raw, &value) != nil {
		return "", false
	}

	for _, character := range value {
		if character > unicode.MaxASCII {
			return value, true
		}
	}

	return photoBase64Text([]byte(value))
}

func photoRecordText(record cloudkit.CKRecord, name string) (string, bool, error) {
	raw, err := reminderField(record, name)
	if err != nil {
		return "", false, err
	}

	wrapperType, err := photoFieldType(record, name)
	if err != nil {
		return "", false, err
	}

	if wrapperType == string(cloudkit.BYTES) || wrapperType == string(cloudkit.ENCRYPTEDBYTES) {
		return photoBinaryText(raw)
	}

	text, ok := photoEncryptedText(raw)

	return text, ok, nil
}

func photoBase64Text(raw []byte) (string, bool) {
	decoded, err := reminderRelatedBase64(string(raw))
	if err == nil && utf8.Valid(decoded) {
		return string(decoded), true
	}

	if utf8.Valid(raw) {
		return string(raw), true
	}

	return "", false
}

func photoFieldType(record cloudkit.CKRecord, name string) (string, error) {
	if record.Fields == nil {
		return "", nil
	}

	field, exists := (*record.Fields)[name]
	if !exists {
		return "", nil
	}

	wrapper, err := field.AsCKPassthroughField()
	if err != nil {
		return "", fmt.Errorf("decode photo text wrapper: %w", err)
	}

	return wrapper.Type, nil
}

func photoBinaryText(raw json.RawMessage) (string, bool, error) {
	var encoded string

	if len(raw) == 0 || string(raw) == jsonNullValue {
		return "", false, nil
	}

	decodeErr := json.Unmarshal(raw, &encoded)
	if decodeErr != nil {
		return "", false, fmt.Errorf("decode photo binary text: %w", decodeErr)
	}

	decoded, err := reminderRelatedBase64(encoded)
	if err != nil {
		return "", false, err
	}

	text, ok := photoBase64Text(decoded)

	return text, ok, nil
}
