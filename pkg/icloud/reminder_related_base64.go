package icloud

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const reminderBase64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func reminderRelatedBase64(encoded string) ([]byte, error) {
	var normalized strings.Builder

	padding := 0

	for index := range len(encoded) {
		character := encoded[index]
		if character == '=' {
			padding++

			position := normalized.Len() % reminderBase64Quantum
			if position >= reminderBase64MinimumPadded && position+padding >= reminderBase64Quantum {
				normalized.WriteString(strings.Repeat("=", reminderBase64Quantum-position))

				break
			}

			continue
		}

		if strings.IndexByte(reminderBase64Alphabet, character) >= 0 {
			padding = 0

			normalized.WriteByte(character)
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(normalized.String())
	if err != nil {
		return nil, fmt.Errorf("decode Source related base64: %w", err)
	}

	return decoded, nil
}

const reminderBase64MinimumPadded = 2
