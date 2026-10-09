package replay

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const compressedJSONRule = "base64-zlib-exact"

func matchFramingLength(actual []string, expected map[string][]string, length int, entity Entity) error {
	err := matchLengthHeader(actual, length)
	if err != nil {
		return err
	}

	if !hasCompressedJSONRule(entity) || len(expected["content-length"]) == 0 {
		return nil
	}

	if len(expected["content-length"]) != 1 {
		return fmt.Errorf("%w: repeated framing length", ErrFixture)
	}

	expected["content-length"] = []string{strconv.Itoa(length)}

	return nil
}

func hasCompressedJSONRule(entity Entity) bool {
	for _, matcher := range entity.Matchers {
		if matcher.Pattern == compressedJSONRule {
			return true
		}
	}

	return false
}

func decodeCompressedJSON(text string) ([]byte, error) {
	if strings.ContainsAny(text, "\r\n") {
		return nil, fmt.Errorf("%w: whitespace in compressed JSON base64", ErrMismatch)
	}

	compressed, err := base64.StdEncoding.Strict().DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("%w: compressed JSON base64: %w", ErrMismatch, err)
	}

	input := bytes.NewReader(compressed)

	reader, err := zlib.NewReader(input)
	if err != nil {
		return nil, fmt.Errorf("%w: compressed JSON stream: %w", ErrMismatch, err)
	}

	decoded, readErr := io.ReadAll(reader)

	closeErr := reader.Close()

	if readErr != nil || closeErr != nil || input.Len() != 0 {
		return nil, fmt.Errorf("%w: invalid or trailing compressed JSON stream", ErrMismatch)
	}

	return decoded, nil
}

func matchCompressedJSON(actual, expected any, matcher JSONMatcher) error {
	actualValue, err := pathValue(actual, matcher.Path)
	if err != nil {
		return err
	}

	expectedValue, err := pathValue(expected, matcher.Path)
	if err != nil {
		return err
	}

	actualText, actualOK := actualValue.(string)

	expectedText, expectedOK := expectedValue.(string)

	if !actualOK || !expectedOK {
		return fmt.Errorf("%w: compressed JSON target must be a string", ErrFixture)
	}

	actualData, err := decodeCompressedJSON(actualText)
	if err != nil {
		return err
	}

	expectedData, err := decodeCompressedJSON(expectedText)
	if err != nil {
		return err
	}

	if !bytes.Equal(actualData, expectedData) {
		return fmt.Errorf("%w: decoded compressed JSON differs", ErrMismatch)
	}

	return nil
}
