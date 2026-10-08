// Package replay supplies offline paired HTTP verification for the iCloud SDK.
package replay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

const base64Encoding = "base64"

var (
	// ErrFixture identifies an unsupported or malformed portable exchange.
	ErrFixture = errors.New("invalid portable HTTP fixture")
	// ErrMismatch identifies rejected traffic, including a previously caught rejection.
	ErrMismatch = errors.New("HTTP replay rejected traffic")
	// ErrUnconsumed identifies pending exchanges or response bodies.
	ErrUnconsumed = errors.New("HTTP replay is not fully consumed")
)

// Pair preserves the order and repeated values of query parameters and headers.
type Pair [2]string

// Entity represents exact binary bytes or an explicit structural JSON request rule.
type Entity struct {
	Encoding string          `json:"encoding"`
	Value    json.RawMessage `json:"value"`
	Matchers []JSONMatcher   `json:"matchers,omitempty"`
}

// Request records the complete prepared request boundary, independently of an SDK.
type Request struct {
	Method  string `json:"method"`
	Origin  string `json:"origin"`
	Path    string `json:"path"`
	Query   []Pair `json:"query"`
	Headers []Pair `json:"headers"`
	Body    Entity `json:"body"`
}

// Response records status, repeated headers and exact entity bytes.
type Response struct {
	Status  int    `json:"status"`
	Headers []Pair `json:"headers"`
	Body    Entity `json:"body"`
}

// Exchange binds one request to its response or a named recorded transport failure.
type Exchange struct {
	Request  Request   `json:"request"`
	Response *Response `json:"response,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// TransportError preserves the reference transport failure class.
type TransportError struct {
	Class string
}

// Error describes an expected synthetic failure without exposing provider data.
func (failure TransportError) Error() string {
	return "recorded transport failure: " + failure.Class
}

// HTTPTransport returns responses only after matching the next complete request.
// Its mutex serializes exchange order; it never forwards traffic to a network.
type HTTPTransport struct {
	mu        sync.Mutex
	exchanges []Exchange
	index     int
	rejection error
	bodies    []*responseBody
}

// NewHTTPTransport takes an immutable snapshot of exact-body exchanges.
// Unsupported match rules fail before an operation can begin.
func NewHTTPTransport(exchanges []Exchange) (*HTTPTransport, error) {
	encoded, err := json.Marshal(exchanges)
	if err != nil {
		return nil, fmt.Errorf("%w: snapshot: %w", ErrFixture, err)
	}

	var snapshot []Exchange

	err = json.Unmarshal(encoded, &snapshot)
	if err != nil {
		return nil, fmt.Errorf("%w: snapshot: %w", ErrFixture, err)
	}

	for index, exchange := range snapshot {
		err = validateExchange(exchange)
		if err != nil {
			return nil, fmt.Errorf("exchange %d: %w", index, err)
		}
	}

	return &HTTPTransport{
		mu: sync.Mutex{}, exchanges: snapshot, index: 0, rejection: nil, bodies: nil,
	}, nil
}

func validateExchange(exchange Exchange) error {
	err := validateTarget(exchange.Request)
	if err != nil {
		return err
	}

	err = validateRequestEntity(exchange.Request.Body)
	if err != nil {
		return err
	}

	if (exchange.Response == nil) == (exchange.Error == "") {
		return fmt.Errorf("%w: exactly one outcome is required", ErrFixture)
	}

	if exchange.Error != "" {
		if !knownFailure(exchange.Error) {
			return fmt.Errorf("%w: unknown transport failure", ErrFixture)
		}

		return nil
	}

	if exchange.Response.Status < 100 || exchange.Response.Status > 599 {
		return fmt.Errorf("%w: response status", ErrFixture)
	}

	_, err = decodeEntity(exchange.Response.Body)

	return err
}

func validateTarget(request Request) error {
	origin, err := url.Parse(request.Origin)
	if err != nil {
		return fmt.Errorf("%w: request origin", ErrFixture)
	}

	if origin.Scheme != "https" || origin.Host == "" || origin.User != nil ||
		origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return fmt.Errorf("%w: request origin", ErrFixture)
	}

	if request.Method == "" || !strings.HasPrefix(request.Path, "/") {
		return fmt.Errorf("%w: request method/path", ErrFixture)
	}

	return nil
}

func knownFailure(class string) bool {
	switch class {
	case "ConnectionError", "Timeout", "ConnectTimeout", "ReadTimeout", "SSLError", "ChunkedEncodingError":
		return true
	default:
		return false
	}
}

func decodeEntity(entity Entity) ([]byte, error) {
	if entity.Encoding != base64Encoding || len(entity.Matchers) != 0 {
		return nil, fmt.Errorf("%w: unsupported entity encoding %q", ErrFixture, entity.Encoding)
	}

	var encoded string

	trimmed := bytes.TrimSpace(entity.Value)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return nil, fmt.Errorf("%w: base64 value must be a string", ErrFixture)
	}

	err := json.Unmarshal(entity.Value, &encoded)
	if err != nil || strings.ContainsAny(encoded, "\r\n") {
		return nil, fmt.Errorf("%w: base64 value", ErrFixture)
	}

	body, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 entity: %w", ErrFixture, err)
	}

	return body, nil
}

// RoundTrip implements http.RoundTripper and retains rejected traffic as a sticky failure.
func (transport *HTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	response, err := transport.send(request)

	closeErr := closeRequestBody(request)
	if closeErr != nil {
		if response != nil {
			closeErr = errors.Join(closeErr, response.Body.Close())
		}

		transport.rejection = fmt.Errorf("%w: request close: %w", ErrMismatch, errors.Join(err, closeErr))

		return nil, transport.rejection
	}

	return response, err
}

// AssertConsumed rejects caught traffic failures, unused exchanges and unread/unclosed bodies.
func (transport *HTTPTransport) AssertConsumed() error {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	if transport.rejection != nil {
		return transport.rejection
	}

	if transport.index != len(transport.exchanges) {
		return fmt.Errorf("%w: %d pending exchanges", ErrUnconsumed, len(transport.exchanges)-transport.index)
	}

	for _, body := range transport.bodies {
		if !body.consumed() {
			return fmt.Errorf("%w: response body", ErrUnconsumed)
		}
	}

	return nil
}

func (transport *HTTPTransport) send(request *http.Request) (*http.Response, error) {
	if transport.rejection != nil {
		return nil, transport.rejection
	}

	if transport.index == len(transport.exchanges) {
		return transport.reject("unexpected or duplicate request")
	}

	exchange := transport.exchanges[transport.index]

	err := matchRequest(request, exchange.Request)
	if err != nil {
		transport.rejection = fmt.Errorf("%w: exchange %d: %w", ErrMismatch, transport.index, err)

		return nil, transport.rejection
	}

	transport.index++

	if exchange.Error != "" {
		return nil, TransportError{Class: exchange.Error}
	}

	return transport.response(request, *exchange.Response)
}

func (transport *HTTPTransport) reject(reason string) (*http.Response, error) {
	transport.rejection = fmt.Errorf("%w: %s", ErrMismatch, reason)

	return nil, transport.rejection
}

func matchRequest(request *http.Request, expected Request) error {
	err := matchTarget(request, expected)
	if err != nil {
		return err
	}

	query, err := orderedQuery(request.URL.RawQuery)
	if err != nil || !reflect.DeepEqual(query, expected.Query) {
		return fmt.Errorf("%w: ordered query", ErrMismatch)
	}

	body, err := requestBody(request)
	if err != nil {
		return err
	}

	if request.ContentLength != int64(len(body)) {
		return fmt.Errorf("%w: content length", ErrMismatch)
	}

	err = matchEntity(body, expected.Body)
	if err != nil {
		return err
	}

	headers := expected.Headers
	if expected.Body.Encoding == redactedEncoding {
		headers = withoutLength(headers)
	}

	return matchHeaders(request, headers, len(body), expected.Body.Encoding == redactedEncoding)
}

func withoutLength(headers []Pair) []Pair {
	result := make([]Pair, 0, len(headers))

	for _, pair := range headers {
		if !strings.EqualFold(pair[0], "Content-Length") {
			result = append(result, pair)
		}
	}

	return result
}

func matchTarget(request *http.Request, expected Request) error {
	if request == nil || request.URL == nil {
		return fmt.Errorf("%w: missing request target", ErrMismatch)
	}

	if request.URL.User != nil || request.URL.Fragment != "" ||
		(request.Host != "" && request.Host != request.URL.Host) {
		return fmt.Errorf("%w: target credentials or framing", ErrMismatch)
	}

	if unsupportedFraming(request) {
		return fmt.Errorf("%w: unsupported target or trailers", ErrMismatch)
	}

	if request.Method != expected.Method || !sameTargetURL(request.URL, expected) {
		return fmt.Errorf("%w: target", ErrMismatch)
	}

	return nil
}

func unsupportedFraming(request *http.Request) bool {
	return request.URL.Opaque != "" || request.URL.ForceQuery ||
		len(request.Trailer) != 0 || len(request.TransferEncoding) != 0 || request.Close
}

func sameTargetURL(target *url.URL, expected Request) bool {
	return target.Scheme+"://"+target.Host == expected.Origin && target.EscapedPath() == expected.Path
}

func requestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: request body read: %w", ErrMismatch, err)
	}

	return body, nil
}

func closeRequestBody(request *http.Request) error {
	if request == nil || request.Body == nil {
		return nil
	}

	err := request.Body.Close()
	if err != nil {
		return fmt.Errorf("close replay request: %w", err)
	}

	return nil
}

func orderedQuery(raw string) ([]Pair, error) {
	result := []Pair{}
	if raw == "" {
		return result, nil
	}

	for item := range strings.SplitSeq(raw, "&") {
		key, value, _ := strings.Cut(item, "=")

		key, err := url.QueryUnescape(key)
		if err != nil {
			return nil, fmt.Errorf("query key: %w", err)
		}

		value, err = url.QueryUnescape(value)
		if err != nil {
			return nil, fmt.Errorf("query value: %w", err)
		}

		result = append(result, Pair{key, value})
	}

	return result, nil
}

func matchHeaders(request *http.Request, expected []Pair, length int, ignoreLength bool) error {
	actual := make(map[string][]string)
	want := make(map[string][]string)

	for key, values := range request.Header {
		actual[strings.ToLower(key)] = append(actual[strings.ToLower(key)], values...)
	}

	for _, pair := range expected {
		key := strings.ToLower(pair[0])
		want[key] = append(want[key], pair[1])
	}

	// Go stores HTTP framing separately from the Header map. Bind the recorded
	// Content-Length to that field and the actual bytes rather than ignoring it.
	if _, required := want["content-length"]; required && actual["content-length"] == nil {
		actual["content-length"] = []string{strconv.FormatInt(request.ContentLength, 10)}
	}

	if lengths := actual["content-length"]; lengths != nil &&
		(len(lengths) != 1 || lengths[0] != strconv.Itoa(length)) {
		return fmt.Errorf("%w: header content length", ErrMismatch)
	}

	if ignoreLength {
		delete(actual, "content-length")
	}

	if !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("%w: headers", ErrMismatch)
	}

	return nil
}

func (transport *HTTPTransport) response(request *http.Request, wire Response) (*http.Response, error) {
	data, err := decodeEntity(wire.Body)
	if err != nil {
		transport.rejection = err

		return nil, err
	}

	body := &responseBody{mu: sync.Mutex{}, reader: bytes.NewReader(data), closed: false}
	transport.bodies = append(transport.bodies, body)
	header := make(http.Header)

	for _, pair := range wire.Headers {
		header.Add(pair[0], pair[1])
	}

	return &http.Response{
		Status:     strconv.Itoa(wire.Status) + " " + http.StatusText(wire.Status),
		StatusCode: wire.Status, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: header, Body: body, ContentLength: int64(len(data)), TransferEncoding: nil,
		Close: false, Uncompressed: false, Trailer: nil, Request: request, TLS: nil,
	}, nil
}

type responseBody struct {
	mu     sync.Mutex
	reader *bytes.Reader
	closed bool
}

func (body *responseBody) Read(buffer []byte) (int, error) {
	body.mu.Lock()
	defer body.mu.Unlock()

	if body.closed {
		return 0, io.ErrClosedPipe
	}

	count, err := body.reader.Read(buffer)
	if errors.Is(err, io.EOF) {
		return count, io.EOF
	}

	if err != nil {
		return count, fmt.Errorf("read replay response: %w", err)
	}

	return count, nil
}

func (body *responseBody) Close() error {
	body.mu.Lock()
	defer body.mu.Unlock()

	body.closed = true

	return nil
}

func (body *responseBody) consumed() bool {
	body.mu.Lock()
	defer body.mu.Unlock()

	return body.closed && body.reader.Len() == 0
}
