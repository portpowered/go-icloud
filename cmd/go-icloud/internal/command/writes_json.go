package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const maxWriteJSONDepth = 128

var (
	errWriteDuplicate = errors.New("write request contains a duplicate object field")
	errWriteNesting   = errors.New("write request JSON nesting exceeds the supported depth")
	errWriteEncoding  = errors.New("write request must use valid UTF-8")
)

func validateWriteJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errWriteEncoding
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkWriteJSON(decoder, 0); err != nil {
		return err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("inspect typed request: %w", err)
	}
	attachment, exists := object["attachment"]
	if !exists {
		return nil
	}

	return validateWriteAttachment(attachment)
}

func walkWriteJSON(decoder *json.Decoder, depth int) error {
	if depth > maxWriteJSONDepth {
		return errWriteNesting
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("inspect JSON value: %w", err)
	}
	opening, composite := token.(json.Delim)
	if !composite {
		return nil
	}

	seen := make(map[string]bool)
	for decoder.More() {
		if opening == '{' {
			key, keyErr := decoder.Token()
			if keyErr != nil {
				return fmt.Errorf("inspect JSON field: %w", keyErr)
			}
			name, _ := key.(string)
			if seen[name] {
				return errWriteDuplicate
			}
			seen[name] = true
		}
		if err = walkWriteJSON(decoder, depth+1); err != nil {
			return err
		}
	}
	if _, err = decoder.Token(); err != nil {
		return fmt.Errorf("inspect JSON boundary: %w", err)
	}

	return nil
}

func validateWriteAttachment(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("inspect attachment variant: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var err error
	if _, selected := fields["url"]; selected {
		var attachment icloud.ReminderURLAttachment
		err = decoder.Decode(&attachment)
	} else {
		var attachment icloud.ReminderImageAttachment
		err = decoder.Decode(&attachment)
	}
	if err != nil {
		return fmt.Errorf("decode attachment variant: %w", err)
	}

	return nil
}
