package replay_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridgewebsocket"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
	"google.golang.org/protobuf/proto"
)

type sdkBridgeNetwork struct {
	PrivateScalars []string `json:"private_scalars"`
	ProverRandom   []struct {
		Upper string `json:"upper_hex"`
		Value string `json:"value_hex"`
	} `json:"prover_random"`
	Connections []struct {
		Connection bridgeSocketConnection `json:"connection"`
		Bootstrap  struct {
			Public     string `json:"public_key"`
			Nonce      string `json:"nonce"`
			Expiration uint32 `json:"expiration_seconds"`
		} `json:"bootstrap"`
		Events []bridgeSocketEvent `json:"events"`
	} `json:"connections"`
	Timeline []map[string]any `json:"timeline"`
}

type sdkBridgeReplay struct {
	t                                    *testing.T
	network                              sdkBridgeNetwork
	http                                 *replay.HTTPTransport
	mu                                   sync.Mutex
	timeline, httpIndex, connectionIndex int
	sockets                              []*sdkBridgeSocket
	entropy                              sdkBridgeEntropy
	session                              *icloud.NativeBridgeSession
	overlap, overlapChecked              bool
}

type sdkBridgeEntropy struct {
	replay                                           *sdkBridgeReplay
	initial                                          []byte
	private, prover                                  int
	signatureReads, nonceReads, wideReads, maskReads int
	signing, uuid                                    bool
}

func (r *sdkBridgeEntropy) Read(destination []byte) (int, error) {
	clear(destination)
	switch len(destination) {
	case 256:
		if len(r.initial) != 256 {
			return 0, errors.New("undeclared SRP entropy")
		}
		copy(destination, r.initial)
		r.initial = nil
	case 1:
		r.signatureReads++
		if r.signatureReads > 4 {
			return 0, errors.New("unbounded optional signature entropy")
		}
		if !r.signing {
			return 0, errors.New("unexpected optional ECDSA entropy")
		}
	case 8:
		r.nonceReads++
		r.uuid = false
		r.signatureReads = 0
		index := r.replay.connectionIndex
		if index >= len(r.replay.network.Connections) {
			return 0, io.EOF
		}
		nonce, err := base64.StdEncoding.DecodeString(r.replay.network.Connections[index].Bootstrap.Nonce)
		if err != nil || len(nonce) != 17 {
			return 0, errors.New("invalid fixture nonce")
		}
		copy(destination, nonce[9:])
		r.signing = true
	case 16:
		r.wideReads++
		if r.wideReads > len(r.replay.network.Connections)+1 {
			return 0, errors.New("undeclared websocket or UUID entropy")
		}
		if r.uuid {
			destination[3] = 1
			r.uuid = false
		} else {
			for index := range destination {
				destination[index] = byte(index)
			}
			r.uuid = true
		}
	case 4:
		r.maskReads++
		for index := range destination {
			destination[index] = byte(index)
		}
	case 32:
		return r.readScalar(destination)
	default:
		return 0, fmt.Errorf("undeclared entropy length %d", len(destination))
	}
	return len(destination), nil
}

func (r *sdkBridgeReplay) take(event map[string]any) {
	r.t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timeline >= len(r.network.Timeline) {
		r.t.Fatalf("extra combined timeline event %v", event)
	}
	if !reflect.DeepEqual(event, r.network.Timeline[r.timeline]) {
		r.t.Fatalf("combined timeline[%d] = %v; expected %v", r.timeline, event, r.network.Timeline[r.timeline])
	}
	r.timeline++
}

func (r *sdkBridgeReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	r.take(map[string]any{"surface": "http", "exchange": float64(r.httpIndex)})
	r.httpIndex++
	return r.http.RoundTrip(request)
}
func (r *sdkBridgeReplay) dial(ctx context.Context, network, address string) (net.Conn, error) {
	deadline, bounded := ctx.Deadline()
	if !bounded || time.Until(deadline) <= 0 || time.Until(deadline) > 30*time.Second {
		return nil, errors.New("secure dial deadline differs")
	}
	index := r.connectionIndex
	if index >= len(r.network.Connections) {
		return nil, errors.New("undeclared bridge connection")
	}
	expected := r.network.Connections[index]
	if network != "tcp" || address != "bridge.example.invalid:443" {
		return nil, errors.New("unexpected secure dial target")
	}
	socket := &sdkBridgeSocket{owner: r, index: index, bridgeScriptSocket: &bridgeScriptSocket{events: append([]bridgeSocketEvent(nil), expected.Events...)}}
	r.sockets = append(r.sockets, socket)
	for _, operation := range []string{"connect", "wrap", "timeout"} {
		socket.mark(operation, 0)
	}
	r.connectionIndex++
	r.entropy.signing = false
	return socket, nil
}

type sdkBridgeSocket struct {
	*bridgeScriptSocket
	owner *sdkBridgeReplay
	index int
}

func (s *sdkBridgeSocket) mark(operation string, event int) {
	value := map[string]any{"surface": "socket", "connection": float64(s.index), "operation": operation}
	if operation == "send" || operation == "receive" || operation == "close" {
		value["event"] = float64(event)
	}
	s.owner.take(value)
}

func (s *sdkBridgeSocket) Write(payload []byte) (int, error) {
	s.mark("send", s.cursor)
	original := len(payload)
	var err error
	if s.cursor == 0 {
		payload, err = s.upgrade(payload)
	} else {
		payload, err = bridgeFixtureUnmask(payload)
	}
	if err != nil {
		return 0, err
	}
	_, err = s.bridgeScriptSocket.Write(payload)
	return original, err
}

func (s *sdkBridgeSocket) Read(destination []byte) (int, error) {
	if len(s.pending) == 0 {
		s.mark("receive", s.cursor)
	}
	return s.bridgeScriptSocket.Read(destination)
}

func (s *sdkBridgeSocket) Close() error {
	if !s.closed {
		s.mark("close", s.cursor)
	}
	return s.bridgeScriptSocket.Close()
}

func (s *sdkBridgeSocket) validateBootstrap(path string) {
	s.owner.t.Helper()
	raw, err := hex.DecodeString(path)
	if err != nil || hex.EncodeToString(raw) != path {
		s.owner.t.Fatal("noncanonical bootstrap path")
	}
	message := new(bridgepb.ClientMessage)
	if err = proto.Unmarshal(raw, message); err != nil {
		s.owner.t.Fatal(err)
	}
	canonical, err := proto.Marshal(message)
	if err != nil || !bytes.Equal(canonical, raw) || message.Connection == nil || message.Subscription != nil || message.Acknowledgement != nil || len(message.ProtoReflect().GetUnknown()) != 0 {
		s.owner.t.Fatal("bootstrap envelope differs")
	}
	body := message.GetConnection()
	expected := s.owner.network.Connections[s.index].Bootstrap
	if base64.StdEncoding.EncodeToString(body.GetPublicKey()) != expected.Public || base64.StdEncoding.EncodeToString(body.GetNonce()) != expected.Nonce || body.GetExpiration().GetSeconds() != expected.Expiration || len(body.ProtoReflect().GetUnknown()) != 0 || len(body.GetExpiration().ProtoReflect().GetUnknown()) != 0 {
		s.owner.t.Fatal("bootstrap public key, nonce, or expiration differs")
	}
	public := body.GetPublicKey()
	nonce := body.GetNonce()
	signature := body.GetSignature()
	if len(public) != 65 || len(nonce) != 17 || nonce[0] != 0 || len(signature) < 2 || !bytes.Equal(signature[:2], []byte{1, 3}) {
		s.owner.t.Fatal("bootstrap field layout differs")
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), public)
	digest := sha256.Sum256(nonce)
	if x == nil || !ecdsa.VerifyASN1(&ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, digest[:], signature[2:]) {
		s.owner.t.Fatal("bootstrap signature rejected")
	}
}

func (r *sdkBridgeEntropy) readScalar(destination []byte) (int, error) {
	var scalar *big.Int
	if r.signing {
		r.signatureReads++
		if r.signatureReads > 4 {
			return 0, errors.New("unbounded signature entropy")
		}
		scalar = big.NewInt(1)
	} else if r.private == 0 {
		if len(r.replay.network.PrivateScalars) != 1 {
			return 0, errors.New("undeclared bootstrap scalar")
		}
		scalar, _ = new(big.Int).SetString(r.replay.network.PrivateScalars[r.private], 16)
		r.private++
	} else {
		if r.prover >= len(r.replay.network.ProverRandom) {
			return 0, errors.New("undeclared prover scalar")
		}
		sample := r.replay.network.ProverRandom[r.prover]
		r.prover++
		upper, ok := new(big.Int).SetString(sample.Upper, 0)
		if !ok || upper.Cmp(elliptic.P256().Params().N) != 0 {
			return 0, errors.New("incorrect prover bound")
		}
		scalar, _ = new(big.Int).SetString(sample.Value, 0)
	}
	if scalar == nil {
		return 0, errors.New("invalid fixture scalar")
	}
	scalar.FillBytes(destination)
	return len(destination), nil
}

func (s *sdkBridgeSocket) upgrade(payload []byte) ([]byte, error) {
	lines := strings.Split(string(payload), "\r\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "GET /v2/") || !strings.HasSuffix(lines[0], " HTTP/1.1") {
		return nil, errors.New("invalid upgrade target")
	}
	path := strings.TrimSuffix(strings.TrimPrefix(lines[0], "GET /v2/"), " HTTP/1.1")
	s.validateBootstrap(path)
	lines[0] = "GET /v2/{signed_bootstrap} HTTP/1.1"
	key := base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	for index, line := range lines {
		if strings.HasPrefix(line, "Sec-WebSocket-Key:") {
			if line != "Sec-WebSocket-Key: "+key {
				return nil, errors.New("websocket entropy differs")
			}
			lines[index] = "Sec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA=="
		}
	}
	digest := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	s.events[1].Template = strings.ReplaceAll(s.events[1].Template, "{accept}", base64.StdEncoding.EncodeToString(digest[:]))
	payload = []byte(strings.Join(lines, "\r\n"))
	return payload, nil
}

func bridgeFixtureUnmask(payload []byte) ([]byte, error) {
	payload = append([]byte(nil), payload...)
	offset := 2
	switch payload[1] & 127 {
	case 126:
		offset += 2
	case 127:
		offset += 8
	}
	if len(payload) < offset+4 || !bytes.Equal(payload[offset:offset+4], []byte{0, 1, 2, 3}) {
		return nil, errors.New("frame mask entropy differs")
	}
	for index := offset + 4; index < len(payload); index++ {
		payload[index] ^= payload[offset+(index-offset-4)%4]
	}
	clear(payload[offset : offset+4])
	return payload, nil
}

// Synthetic caller cancellation occurs after the complete Source prompt boundary.
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
type bridgeScriptSocket struct {
	mu      sync.Mutex
	events  []bridgeSocketEvent
	pending []byte
	cursor  int
	closed  bool
	failure error
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
