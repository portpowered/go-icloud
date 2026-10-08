package replay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

const multipartEncoding = "multipart"

// MultipartPart binds ordered part headers and exact binary bytes.
type MultipartPart struct {
	Headers []Pair `json:"headers"`
	Body    Entity `json:"body"`
}

func validateMultipartEntity(entity Entity) error {
	if ambiguousMultipartRule(entity) {
		return fmt.Errorf("%w: incomplete or ambiguous multipart rule", ErrFixture)
	}

	_, err := regexp.Compile("^(?:" + entity.ContentTypePattern + ")$")
	if err != nil {
		return fmt.Errorf("%w: multipart pattern: %w", ErrFixture, err)
	}

	for _, part := range entity.Parts {
		if len(part.Headers) == 0 {
			return fmt.Errorf("%w: multipart part headers", ErrFixture)
		}

		for _, header := range part.Headers {
			if !validPartHeader(header) {
				return fmt.Errorf("%w: multipart header", ErrFixture)
			}
		}

		_, err = decodeEntity(part.Body)
		if err != nil {
			return err
		}
	}

	return nil
}

func ambiguousMultipartRule(entity Entity) bool {
	return len(entity.Value) != 0 || len(entity.Matchers) != 0 || len(entity.Parts) == 0 || entity.ContentTypePattern == ""
}

func validPartHeader(header Pair) bool {
	if header[0] == "" || strings.ContainsAny(header[1], "\r\n") || !utf8.ValidString(header[1]) {
		return false
	}

	for _, character := range header[0] {
		if !headerCharacter(character) {
			return false
		}
	}

	return supportedPartHeader(header)
}

func headerCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", character)
}

func supportedPartHeader(header Pair) bool {
	if strings.EqualFold(header[0], "Content-Transfer-Encoding") {
		return false
	}

	if strings.EqualFold(header[0], "Content-Type") && strings.HasPrefix(strings.ToLower(header[1]), "multipart/") {
		return false
	}

	return true
}

func validateMultipartHeaders(request Request) error {
	if request.Body.Encoding != multipartEncoding {
		return nil
	}

	header := make(http.Header)

	for _, pair := range request.Headers {
		header.Add(pair[0], pair[1])
	}

	_, err := multipartBoundary(header, request.Body.ContentTypePattern)
	if err != nil {
		return fmt.Errorf("%w: declared content type: %w", ErrFixture, err)
	}

	return nil
}

func multipartBoundary(header http.Header, expression string) (string, error) {
	values := []string{}

	for key, items := range header {
		if strings.EqualFold(key, "Content-Type") {
			values = append(values, items...)
		}
	}

	if len(values) != 1 {
		return "", fmt.Errorf("%w: multipart content type count", ErrMismatch)
	}

	pattern, err := regexp.Compile("^(?:" + expression + ")$")
	if err != nil || !pattern.MatchString(values[0]) {
		return "", fmt.Errorf("%w: multipart content type format", ErrMismatch)
	}

	mediaType, parameters, err := mime.ParseMediaType(values[0])
	if err != nil || mediaType != "multipart/form-data" || parameters["boundary"] == "" {
		return "", fmt.Errorf("%w: multipart boundary", ErrMismatch)
	}

	return parameters["boundary"], nil
}

func matchMultipart(body []byte, header http.Header, expected Entity) error {
	boundary, err := multipartBoundary(header, expected.ContentTypePattern)
	if err != nil {
		return err
	}

	parts, err := splitMultipart(body, boundary)
	if err != nil || len(parts) != len(expected.Parts) {
		return fmt.Errorf("%w: multipart framing or part count", ErrMismatch)
	}

	for index, part := range parts {
		if !reflect.DeepEqual(part.Headers, expected.Parts[index].Headers) {
			return fmt.Errorf("%w: multipart part headers", ErrMismatch)
		}

		data, err := decodeEntity(part.Body)
		if err != nil {
			return err
		}

		err = matchEntity(data, expected.Parts[index].Body)
		if err != nil {
			return err
		}
	}

	return nil
}

func splitMultipart(body []byte, boundary string) ([]MultipartPart, error) {
	prefix := []byte("--" + boundary + "\r\n")

	suffix := []byte("\r\n--" + boundary + "--\r\n")
	if len(body) < len(prefix)+len(suffix) || !bytes.HasPrefix(body, prefix) || !bytes.HasSuffix(body, suffix) {
		return nil, fmt.Errorf("%w: multipart preamble/epilogue or closing boundary", ErrMismatch)
	}

	content := body[len(prefix) : len(body)-len(suffix)]
	separator := []byte("\r\n--" + boundary + "\r\n")
	parts := []MultipartPart{}

	for block := range bytes.SplitSeq(content, separator) {
		if containsPartBoundary(block, boundary) {
			return nil, fmt.Errorf("%w: unexpected interior boundary", ErrMismatch)
		}

		part, err := decodePart(block)
		if err != nil {
			return nil, err
		}

		parts = append(parts, part)
	}

	return parts, nil
}

func containsPartBoundary(block []byte, boundary string) bool {
	marker := []byte("\r\n--" + boundary)

	for {
		_, remainder, found := bytes.Cut(block, marker)
		if !found {
			return false
		}

		if len(remainder) == 0 || bytes.HasPrefix(remainder, []byte("--")) ||
			bytes.HasPrefix(remainder, []byte("\r\n")) || remainder[0] == ' ' || remainder[0] == '\t' {
			return true
		}

		block = remainder
	}
}

func binaryEntity(body []byte) Entity {
	return Entity{
		Encoding: base64Encoding, Value: json.RawMessage(`"` + base64.StdEncoding.EncodeToString(body) + `"`),
		Matchers: nil, ContentTypePattern: "", Parts: nil,
	}
}

func decodePart(block []byte) (MultipartPart, error) {
	var zero MultipartPart

	headers, body, found := bytes.Cut(block, []byte("\r\n\r\n"))
	if !found {
		return zero, fmt.Errorf("%w: multipart header separator", ErrMismatch)
	}

	parts := []Pair{}

	for line := range bytes.SplitSeq(headers, []byte("\r\n")) {
		key, value, present := bytes.Cut(line, []byte(":"))

		pair := Pair{strings.ToLower(string(key)), strings.TrimLeft(string(value), " \t")}
		if !present || !validPartHeader(pair) {
			return zero, fmt.Errorf("%w: multipart header syntax", ErrMismatch)
		}

		parts = append(parts, pair)
	}

	return MultipartPart{Headers: parts, Body: binaryEntity(body)}, nil
}
