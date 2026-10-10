package bridgeprover

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"
)

const (
	base64Quantum        = 4
	minimumPaddedSymbols = 2
)

// decodeBase64 mirrors Python's non-validating b64decode after ASCII encoding.
// It discards ASCII punctuation, permits noncanonical pad bits and trailing data
// after terminal padding, and still rejects missing padding and non-ASCII input.
func decodeBase64(value string) ([]byte, error) {
	clean, err := cleanBase64(value)
	if err != nil {
		return nil, err
	}

	output, quad := []byte{}, []byte{}
	pads := 0

	for index := range len(clean) {
		character := clean[index]
		if character == '=' {
			pads++
			if len(quad) >= minimumPaddedSymbols && len(quad)+pads >= base64Quantum {
				quad = append(quad, []byte(strings.Repeat("=", base64Quantum-len(quad)))...)

				decoded, err := base64.StdEncoding.DecodeString(string(quad))
				if err != nil {
					return nil, fmt.Errorf("bridge base64 padding: %w", err)
				}

				return append(output, decoded...), nil
			}

			continue
		}

		pads = 0

		quad = append(quad, character)
		if len(quad) == base64Quantum {
			decoded, err := base64.StdEncoding.DecodeString(string(quad))
			if err != nil {
				return nil, fmt.Errorf("bridge base64 quartet: %w", err)
			}

			output = append(output, decoded...)
			quad = quad[:0]
		}
	}

	if len(quad) != 0 {
		return nil, fmt.Errorf("bridge base64 padding: %w", ErrPayload)
	}

	return output, nil
}

func base64Character(character rune) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9' || character == '+' || character == '/'
}

func cleanBase64(value string) (string, error) {
	for _, character := range value {
		if character > unicode.MaxASCII {
			return "", ErrPayload
		}
	}

	clean := strings.Map(func(character rune) rune {
		if base64Character(character) || character == '=' {
			return character
		}

		return -1
	}, value)

	return clean, nil
}
