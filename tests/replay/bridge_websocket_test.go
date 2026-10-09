package replay_test

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // RFC 6455 requires this digest for upgrade acceptance.
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
)

type bridgeSocketFixture struct {
	Operation  string                 `json:"operation"`
	Connection bridgeSocketConnection `json:"connection"`
	Events     []bridgeSocketEvent    `json:"events"`
	Actions    []bridgeSocketAction   `json:"actions"`
	Results    json.RawMessage        `json:"results"`
	Error      json.RawMessage        `json:"error"`
}
type bridgeSocketConnection struct {
	URL       string `json:"url"`
	Origin    string `json:"origin"`
	UserAgent string `json:"user_agent"` //nolint:tagliatelle // The pinned Source fixture uses snake_case.
}
type bridgeSocketEvent struct {
	Direction string   `json:"direction"`
	Kind      string   `json:"kind"`
	Lines     []string `json:"lines"`
	Suffix    string   `json:"suffix"`
	Template  string   `json:"template"`
	Data      string   `json:"data"`
	Opcode    byte     `json:"opcode"`
	Payload   string   `json:"payload"`
	Error     string   `json:"error"`
}
type bridgeSocketAction struct {
	Operation string `json:"operation"`
	Payload   string `json:"payload"`
}

// The scripted connection consumes the existing source-derived socket transcript
// in strict order. It never supplies a fallback response after a mismatch.
type bridgeScriptSocket struct {
	mu      sync.Mutex
	events  []bridgeSocketEvent
	pending []byte
	cursor  int
	closed  bool
	failure error
}

func TestBridgeWebSocketPairedTranscripts(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/socket/*.json")
	if err != nil {
		t.Fatal(err)
	}

	covered := 0

	for _, path := range paths {
		fixture := readBridgeSocketFixture(t, path)
		if !rawBridgeSocketFixture(fixture) {
			continue
		}

		covered++

		t.Run(fixture.Operation, func(t *testing.T) { t.Parallel(); runBridgeSocketFixture(t, fixture) })
	}

	if covered != 18 {
		t.Fatalf("raw transcript inventory changed: %d", covered)
	}
}

func readBridgeSocketFixture(t *testing.T, path string) bridgeSocketFixture {
	t.Helper()

	//nolint:gosec // Paths come exclusively from the fixed canonical fixture glob.
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	fixture := new(bridgeSocketFixture)

	err = json.Unmarshal(payload, fixture)
	if err != nil {
		t.Fatal(err)
	}

	return *fixture
}

func rawBridgeSocketFixture(fixture bridgeSocketFixture) bool {
	for _, action := range fixture.Actions {
		if action.Operation != "read_message" && action.Operation != "send_binary" {
			return false
		}
	}

	return true
}

func runBridgeSocketFixture(t *testing.T, fixture bridgeSocketFixture) {
	t.Helper()

	socket := new(bridgeScriptSocket)
	socket.events = fixture.Events
	options := new(bridgewebsocket.Options)
	options.URL = fixture.Connection.URL
	options.Origin, options.UserAgent = fixture.Connection.Origin, fixture.Connection.UserAgent
	options.Random = bytes.NewReader(make([]byte, 256))
	options.Dial = func(context.Context, string, string) (net.Conn, error) { return socket, nil }
	conn, operationError := bridgewebsocket.Open(t.Context(), *options)

	results := []string{}

	if operationError == nil {
		results, operationError = runBridgeSocketActions(t, conn, fixture.Actions)

		err := conn.Close()
		if err != nil {
			t.Fatal(err)
		}

		err = conn.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	expectedError := len(fixture.Error) > 0
	if (operationError != nil) != expectedError {
		t.Fatalf("operation error = %v, expected error %t", operationError, expectedError)
	}

	expectedResults := []string{}

	err := json.Unmarshal(fixture.Results, &expectedResults)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal([]byte(strings.Join(results, "|")), []byte(strings.Join(expectedResults, "|"))) {
		t.Fatalf("results = %v; want %v", results, fixture.Results)
	}

	socket.mu.Lock()
	defer socket.mu.Unlock()

	if socket.failure != nil {
		t.Fatal(socket.failure)
	}

	if socket.cursor != len(socket.events) {
		t.Fatalf("consumed %d/%d socket events", socket.cursor, len(socket.events))
	}

	if !socket.closed {
		t.Fatal("socket was not closed")
	}
}

func runBridgeSocketActions(t *testing.T, conn *bridgewebsocket.Conn, actions []bridgeSocketAction) ([]string, error) {
	t.Helper()

	results := []string{}

	for _, action := range actions {
		if action.Operation == "send_binary" {
			payload, err := base64.StdEncoding.DecodeString(action.Payload)
			if err != nil {
				t.Fatal(err)
			}

			err = conn.SendBinary(t.Context(), payload)
			if err != nil {
				return results, fmt.Errorf("socket action: %w", err)
			}
		} else {
			payload, err := conn.ReadMessage(t.Context())
			if err != nil {
				return results, fmt.Errorf("socket action: %w", err)
			}

			results = append(results, base64.StdEncoding.EncodeToString(payload))
		}
	}

	return results, nil
}

func (s *bridgeScriptSocket) Write(payload []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, net.ErrClosed
	}

	if s.cursor >= len(s.events) {
		return 0, s.fail("unexpected socket send after transcript")
	}

	event := s.events[s.cursor]
	if event.Direction != "send" {
		return 0, s.fail("socket send out of order")
	}

	expected, err := expectedBridgeSend(event)
	if err != nil {
		return 0, err
	}

	if !bytes.Equal(payload, expected) {
		return 0, s.fail(fmt.Sprintf("socket send mismatch at event %d", s.cursor))
	}

	s.cursor++

	if event.Error != "" {
		return 0, fmt.Errorf("%s: %w", event.Error, io.ErrClosedPipe)
	}

	return len(payload), nil
}

func expectedBridgeSend(event bridgeSocketEvent) ([]byte, error) {
	if event.Kind == "upgrade" {
		lines := append([]string(nil), event.Lines...)
		lines = append(lines, "Sec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA==", "", "")

		return []byte(strings.Join(lines, "\r\n")), nil
	}

	payload, err := base64.StdEncoding.DecodeString(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("fixture payload: %w", err)
	}

	header := []byte{0x80 | event.Opcode}
	length := len(payload)

	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length < 65536:
		header = binary.BigEndian.AppendUint16(append(header, 0x80|126), uint16(length))
	default:
		header = binary.BigEndian.AppendUint64(append(header, 0x80|127), uint64(length))
	}

	header = append(header, 0, 0, 0, 0)

	return append(header, payload...), nil
}

func (s *bridgeScriptSocket) Read(destination []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, net.ErrClosed
	}

	if len(s.pending) == 0 {
		if s.cursor >= len(s.events) {
			return 0, s.fail("unexpected socket read after transcript")
		}

		event := s.events[s.cursor]
		if event.Direction != "receive" {
			return 0, s.fail("socket receive out of order")
		}

		s.cursor++

		var err error

		s.pending, err = expectedBridgeReceive(event)
		if err != nil {
			return 0, err
		}

		if len(s.pending) == 0 {
			return 0, io.EOF
		}
	}

	count := copy(destination, s.pending)
	s.pending = s.pending[count:]

	return count, nil
}

func expectedBridgeReceive(event bridgeSocketEvent) ([]byte, error) {
	if event.Kind == "upgrade" {
		//nolint:gosec // The pinned RFC 6455 fixture uses SHA-1 for its accept digest.
		digest := sha1.Sum([]byte("AAAAAAAAAAAAAAAAAAAAAA==258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		accept := base64.StdEncoding.EncodeToString(digest[:])

		suffix, err := base64.StdEncoding.DecodeString(event.Suffix)
		if err != nil {
			return nil, fmt.Errorf("fixture socket suffix: %w", err)
		}

		return append([]byte(strings.ReplaceAll(event.Template, "{accept}", accept)), suffix...), nil
	}

	payload, err := base64.StdEncoding.DecodeString(event.Data)
	if err != nil {
		return nil, fmt.Errorf("fixture socket bytes: %w", err)
	}

	return payload, nil
}

func (s *bridgeScriptSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	if s.cursor < len(s.events) && s.events[s.cursor].Direction == "close" {
		s.cursor++
	}

	return nil
}
func (*bridgeScriptSocket) LocalAddr() net.Addr              { return nil }
func (*bridgeScriptSocket) RemoteAddr() net.Addr             { return nil }
func (*bridgeScriptSocket) SetDeadline(time.Time) error      { return nil }
func (*bridgeScriptSocket) SetReadDeadline(time.Time) error  { return nil }
func (*bridgeScriptSocket) SetWriteDeadline(time.Time) error { return nil }

func (s *bridgeScriptSocket) fail(message string) error {
	s.failure = fmt.Errorf("%s: %w", message, bridgewebsocket.ErrProtocol)

	return s.failure
}
