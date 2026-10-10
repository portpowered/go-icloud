package webtransport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"unicode"
	"unicode/utf16"
)

// referenceJSONFields preserves Source field order when a generated extensible model uses map marshaling.
func referenceJSONFields(value any, fields []string) ([]byte, error) {
	encoded, err := referenceJSON(value)
	if err != nil {
		return nil, err
	}

	var object map[string]json.RawMessage

	err = json.Unmarshal(encoded, &object)
	if err != nil {
		return nil, fmt.Errorf("decode ordered request object: %w", err)
	}

	ordered := []string{}

	for _, field := range fields {
		if _, exists := object[field]; exists {
			ordered = append(ordered, field)
		}
	}

	remaining := maps.Clone(object)
	for _, field := range ordered {
		delete(remaining, field)
	}

	ordered = append(ordered, slices.Sorted(maps.Keys(remaining))...)

	var output bytes.Buffer

	output.WriteByte('{')

	for index, field := range ordered {
		if index != 0 {
			output.WriteByte(',')
		}

		key, encodeErr := json.Marshal(field)
		if encodeErr != nil {
			return nil, fmt.Errorf("encode ordered request field: %w", encodeErr)
		}

		output.Write(key)
		output.WriteByte(':')
		output.Write(object[field])
	}

	output.WriteByte('}')

	var compact bytes.Buffer

	err = json.Compact(&compact, output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("compact ordered request object: %w", err)
	}

	return referenceJSONSpacing(compact.Bytes()), nil
}

// referenceJSON preserves the pinned client's JSON spacing and ASCII string encoding.
// Generated request models own field names and their construction order.
func referenceJSON(value any) ([]byte, error) {
	var encoded bytes.Buffer

	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(value)
	if err != nil {
		return nil, fmt.Errorf("encode web request: %w", err)
	}

	return referenceJSONSpacing(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'})), nil
}

func referenceJSONSpacing(encoded []byte) []byte {
	var formatted bytes.Buffer

	quoted, escaped := false, false

	for _, character := range string(encoded) {
		writeASCII(&formatted, character)

		if escaped {
			escaped = false

			continue
		}

		switch {
		case quoted && character == '\\':
			escaped = true
		case character == '"':
			quoted = !quoted
		case !quoted && (character == ',' || character == ':'):
			formatted.WriteByte(' ')
		}
	}

	return formatted.Bytes()
}

func writeASCII(output *bytes.Buffer, character rune) {
	if character < unicode.MaxASCII {
		output.WriteRune(character)

		return
	}

	for _, code := range utf16.Encode([]rune{character}) {
		fmt.Fprintf(output, "\\u%04x", code)
	}
}
