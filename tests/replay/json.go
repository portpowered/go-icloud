package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	patternEncoding  = "json-pattern"
	redactedEncoding = "json-redacted"
	nonemptyMarker   = "<redacted:nonempty-password>"
	codeDigits       = 6
	codeMarker       = "<redacted:six-digit-code>"
)

// JSONMatcher binds one string at a nonempty object/array path to a full-match pattern.
type JSONMatcher struct {
	Path    []json.RawMessage `json:"path"`
	Pattern string            `json:"pattern"`
}

// UnmarshalJSON rejects unknown rule fields rather than silently weakening a matcher.
func (entity *Entity) UnmarshalJSON(data []byte) error {
	type entityJSON Entity

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var decoded entityJSON

	err := validateJSONKeys(data)
	if err != nil {
		return err
	}

	err = decoder.Decode(&decoded)
	if err != nil {
		return fmt.Errorf("%w: entity declaration: %w", ErrFixture, err)
	}

	*entity = Entity(decoded)

	return nil
}

func decodeJSON(data []byte) (any, error) {
	err := validateJSONKeys(data)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any

	err = decoder.Decode(&value)
	if err != nil {
		return nil, fmt.Errorf("%w: JSON entity: %w", ErrFixture, err)
	}

	var trailing any

	err = decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing JSON data", ErrFixture)
	}

	return value, nil
}

func validateJSONKeys(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%w: invalid UTF-8", ErrFixture)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	err := uniqueJSONValue(decoder)
	if err != nil {
		return err
	}

	_, err = decoder.Token()
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing JSON tokens", ErrFixture)
	}

	return nil
}

func uniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: JSON token: %w", ErrFixture, err)
	}

	if token == json.Delim('{') {
		return uniqueJSONObject(decoder)
	}

	if token == json.Delim('[') {
		for decoder.More() {
			err = uniqueJSONValue(decoder)
			if err != nil {
				return err
			}
		}

		_, err = decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: JSON array end: %w", ErrFixture, err)
		}
	}

	return nil
}

func uniqueJSONObject(decoder *json.Decoder) error {
	keys := make(map[string]bool)

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: JSON key: %w", ErrFixture, err)
		}

		key, ok := token.(string)
		if !ok || keys[key] {
			return fmt.Errorf("%w: duplicate or invalid JSON key", ErrFixture)
		}

		keys[key] = true

		err = uniqueJSONValue(decoder)
		if err != nil {
			return err
		}
	}

	_, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: JSON object end: %w", ErrFixture, err)
	}

	return nil
}

func validateRequestEntity(entity Entity) error {
	if entity.Encoding != multipartEncoding && (entity.ContentTypePattern != "" || len(entity.Parts) != 0) {
		return fmt.Errorf("%w: unexpected multipart fields", ErrFixture)
	}

	switch entity.Encoding {
	case base64Encoding:
		_, err := decodeEntity(entity)

		return err
	case patternEncoding:
		return validateMatchers(entity)
	case redactedEncoding:
		return validateRedactedEntity(entity)
	case multipartEncoding:
		return validateMultipartEntity(entity)
	default:
		return fmt.Errorf("%w: unsupported request encoding", ErrFixture)
	}
}

func validateRedactedEntity(entity Entity) error {
	if len(entity.Matchers) != 0 {
		return fmt.Errorf("%w: redaction cannot include matchers", ErrFixture)
	}

	value, err := decodeJSON(entity.Value)
	if err != nil {
		return err
	}

	count, err := validateRedactions(value)
	if err != nil || count == 0 {
		return fmt.Errorf("%w: missing or invalid redaction declaration", ErrFixture)
	}

	return nil
}

func validateMatchers(entity Entity) error {
	if len(entity.Matchers) == 0 {
		return fmt.Errorf("%w: empty matcher list", ErrFixture)
	}

	value, err := decodeJSON(entity.Value)
	if err != nil {
		return err
	}

	seen := make(map[string]bool)

	for _, matcher := range entity.Matchers {
		if len(matcher.Path) == 0 || matcher.Pattern == "" {
			return fmt.Errorf("%w: empty matcher path", ErrFixture)
		}

		identity, err := canonicalPath(matcher.Path)
		if err != nil {
			return err
		}

		if seen[identity] {
			return fmt.Errorf("%w: duplicate matcher path", ErrFixture)
		}

		seen[identity] = true

		err = checkPattern(value, matcher)
		if err != nil {
			return fmt.Errorf("%w: invalid matcher: %w", ErrFixture, err)
		}
	}

	return nil
}

func canonicalPath(path []json.RawMessage) (string, error) {
	segments := make([]any, 0, len(path))

	for _, segment := range path {
		value, err := decodeJSON(segment)
		if err != nil {
			return "", err
		}

		value, err = canonicalSegment(value)
		if err != nil {
			return "", err
		}

		segments = append(segments, value)
	}

	encoded, err := json.Marshal(segments)
	if err != nil {
		return "", fmt.Errorf("%w: canonical path: %w", ErrFixture, err)
	}

	return string(encoded), nil
}

func canonicalSegment(value any) (any, error) {
	switch segment := value.(type) {
	case string:
		return segment, nil
	case json.Number:
		index, err := strconv.Atoi(string(segment))
		if err != nil || index < 0 {
			return nil, fmt.Errorf("%w: path index", ErrFixture)
		}

		return index, nil
	default:
		return nil, fmt.Errorf("%w: path must contain strings or nonnegative indices", ErrFixture)
	}
}

func checkPattern(value any, matcher JSONMatcher) error {
	item, err := pathValue(value, matcher.Path)
	if err != nil {
		return err
	}

	text, ok := item.(string)
	if !ok {
		return fmt.Errorf("%w: matcher target is not a string", ErrFixture)
	}

	pattern, err := regexp.Compile("^(?:" + matcher.Pattern + ")$")
	if err != nil {
		return fmt.Errorf("%w: pattern: %w", ErrFixture, err)
	}

	if !pattern.MatchString(text) {
		return fmt.Errorf("%w: string format", ErrMismatch)
	}

	return nil
}

func pathValue(value any, path []json.RawMessage) (any, error) {
	for _, segment := range path {
		var err error

		value, err = childValue(value, segment)
		if err != nil {
			return nil, err
		}
	}

	return value, nil
}

func childValue(value any, segment json.RawMessage) (any, error) {
	switch container := value.(type) {
	case map[string]any:
		var key string

		err := stringSegment(segment, &key)
		if err != nil {
			return nil, fmt.Errorf("%w: object path key", ErrFixture)
		}

		child, found := container[key]
		if !found {
			return nil, fmt.Errorf("%w: missing path key", ErrFixture)
		}

		return child, nil
	case []any:
		index, err := strconv.Atoi(string(segment))
		if err != nil || index < 0 || index >= len(container) {
			return nil, fmt.Errorf("%w: array path index", ErrFixture)
		}

		return container[index], nil
	default:
		return nil, fmt.Errorf("%w: path parent is not a container", ErrFixture)
	}
}

func matchEntity(body []byte, expected Entity) error {
	if expected.Encoding == base64Encoding {
		want, err := decodeEntity(expected)
		if err != nil || !bytes.Equal(body, want) {
			return fmt.Errorf("%w: exact entity", ErrMismatch)
		}

		return nil
	}

	actual, err := decodeJSON(body)
	if err != nil {
		return fmt.Errorf("%w: request JSON: %w", ErrMismatch, err)
	}

	want, err := decodeJSON(expected.Value)
	if err != nil {
		return err
	}

	if expected.Encoding == redactedEncoding {
		_, err = redactJSON(actual, false)
	} else {
		err = replacePatterns(actual, want, expected.Matchers)
	}

	if err != nil || !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("%w: structural entity", ErrMismatch)
	}

	return nil
}

func replacePatterns(actual, expected any, matchers []JSONMatcher) error {
	for _, matcher := range matchers {
		err := checkPattern(actual, matcher)
		if err != nil {
			return err
		}
	}

	for _, matcher := range matchers {
		parent, err := pathValue(actual, matcher.Path[:len(matcher.Path)-1])
		if err != nil {
			return err
		}

		sample, err := pathValue(expected, matcher.Path)
		if err != nil {
			return err
		}

		err = setChild(parent, matcher.Path[len(matcher.Path)-1], sample)
		if err != nil {
			return err
		}
	}

	return nil
}

func setChild(parent any, segment json.RawMessage, sample any) error {
	switch container := parent.(type) {
	case map[string]any:
		var key string

		err := stringSegment(segment, &key)
		if err != nil {
			return fmt.Errorf("%w: object path key", ErrFixture)
		}

		container[key] = sample
	case []any:
		index, err := strconv.Atoi(string(segment))
		if err != nil || index < 0 || index >= len(container) {
			return fmt.Errorf("%w: array path index", ErrFixture)
		}

		container[index] = sample
	default:
		return fmt.Errorf("%w: path parent", ErrFixture)
	}

	return nil
}

func stringSegment(segment json.RawMessage, key *string) error {
	trimmed := bytes.TrimSpace(segment)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return fmt.Errorf("%w: path segment is not a string", ErrFixture)
	}

	err := json.Unmarshal(segment, key)
	if err != nil {
		return fmt.Errorf("%w: path segment: %w", ErrFixture, err)
	}

	return nil
}

func validateRedactions(value any) (int, error) {
	return redactJSON(value, true)
}

func redactJSON(value any, declaration bool) (int, error) {
	switch container := value.(type) {
	case map[string]any:
		return redactObject(container, declaration)
	case []any:
		count := 0

		for _, child := range container {
			nested, err := redactJSON(child, declaration)
			if err != nil {
				return 0, err
			}

			count += nested
		}

		return count, nil
	default:
		return 0, nil
	}
}

func redactObject(container map[string]any, declaration bool) (int, error) {
	count := 0

	for key, value := range container {
		marker := redactionMarker(key)
		if marker != "" {
			text, ok := value.(string)
			if !ok || !validSecret(text, marker, declaration) {
				return 0, fmt.Errorf("%w: invalid redacted field", ErrFixture)
			}

			container[key] = marker
			count++

			continue
		}

		nested, err := redactJSON(value, declaration)
		if err != nil {
			return 0, err
		}

		count += nested
	}

	return count, nil
}

func redactionMarker(key string) string {
	switch strings.ToLower(key) {
	case "password":
		return nonemptyMarker
	case "code", "verificationcode":
		return codeMarker
	default:
		return ""
	}
}

func validSecret(text, marker string, declaration bool) bool {
	if declaration {
		return text == marker
	}

	if marker == nonemptyMarker {
		return text != ""
	}

	if len(text) != codeDigits {
		return false
	}

	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}
