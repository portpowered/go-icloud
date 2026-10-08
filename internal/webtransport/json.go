package webtransport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode"
	"unicode/utf16"
)

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

	var formatted bytes.Buffer

	quoted, escaped := false, false

	for _, character := range string(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'})) {
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

	return formatted.Bytes(), nil
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
